package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"ukt-payment-system/internal/auth"
	"ukt-payment-system/internal/config"
	"ukt-payment-system/internal/database"
	"ukt-payment-system/internal/institusi"
	"ukt-payment-system/internal/middleware"
	"ukt-payment-system/internal/tagihan"
	"ukt-payment-system/internal/webhook"
	"ukt-payment-system/pkg/response"
)

func main() {
	migrateOnly := flag.Bool("migrate-only", false, "Hanya buat database, jalankan migrasi & seeder, lalu keluar")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] Gagal memuat konfigurasi aplikasi: %v", err)
	}

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx := context.Background()

	// 1. Pastikan database target tersedia pada instance PostgreSQL, lalu buka connection pool
	if err := database.EnsureDatabaseExists(ctx, cfg.AdminDSN(), cfg.DBName); err != nil {
		log.Printf("[WARN] Lewati pembuatan database otomatis: %v", err)
	}

	pool, err := database.NewPostgresPool(ctx, cfg.DSN())
	if err != nil {
		log.Fatalf("[FATAL] %v", err)
	}
	defer pool.Close()

	// 2. Jalankan migrasi skema otomatis & seeder 3 institusi
	if err := database.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("[FATAL] %v", err)
	}
	if err := database.SeedInitialData(ctx, pool); err != nil {
		log.Fatalf("[FATAL] %v", err)
	}

	if *migrateOnly {
		log.Printf("[MIGRATE-ONLY] Database '%s' di %s:%s berhasil dibuat, dimigrasi, dan di-seed!", cfg.DBName, cfg.DBHost, cfg.DBPort)
		return
	}

	// 3. Inisialisasi Repository, Service, dan Handler
	instRepo := institusi.NewPostgresRepository(pool)
	authRepo := auth.NewPostgresRepository(pool)
	tagihanRepo := tagihan.NewPostgresRepository(pool)
	webhookRepo := webhook.NewPostgresRepository(pool)

	authSvc := auth.NewService(authRepo, cfg.JWTSecret, cfg.JWTTTL)
	tagihanSvc := tagihan.NewService(tagihanRepo)
	webhookSvc := webhook.NewService(webhookRepo, instRepo, cfg.ResolveHMACSecret)

	authHandler := auth.NewHandler(authSvc)
	tagihanHandler := tagihan.NewHandler(tagihanSvc)
	webhookHandler := webhook.NewHandler(webhookSvc)

	// 4. Konfigurasi Router Gin
	router := gin.Default()
	router.Use(middleware.CORSMiddleware())

	api := router.Group("/api")
	{
		// Healthcheck
		api.GET("/healthz", func(c *gin.Context) {
			if err := pool.Ping(c.Request.Context()); err != nil {
				response.Error(c, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Koneksi database PostgreSQL terputus")
				return
			}
			response.OK(c, gin.H{
				"service":   "ukt-multi-institution-api",
				"database":  "connected",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		})

		// Daftar 3 institusi (publik, untuk informasi di UI / simulator tanpa membuka secret HMAC)
		api.GET("/institutions", func(c *gin.Context) {
			list, err := instRepo.ListAll(c.Request.Context())
			if err != nil {
				response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal mengambil daftar institusi")
				return
			}
			response.OK(c, list)
		})

		// Autentikasi Login
		api.POST("/auth/login", authHandler.Login)

		// Endpoint penerima Webhook dari Payment Gateway (Verifikasi X-Signature HMAC-SHA256)
		api.POST("/webhooks/payment", webhookHandler.ReceivePaymentWebhook)

		// Endpoint Simulator Mock Payment Gateway (untuk mempermudah pengujian interaktif & demo)
		api.GET("/mock-gateway/invoices", webhookHandler.ListSimulatorInvoices)
		api.POST("/mock-gateway/trigger", webhookHandler.TriggerMockWebhook)
		api.GET("/mock-gateway/logs", webhookHandler.ListWebhookLogs)

		// Endpoint Terproteksi JWT
		protected := api.Group("")
		protected.Use(middleware.AuthRequired(cfg.JWTSecret))
		{
			protected.GET("/me", authHandler.Me)

			// Endpoint khusus Admin Keuangan (Diisolasi ketat per institusi_id dari klaim JWT)
			adminGroup := protected.Group("/admin")
			adminGroup.Use(middleware.RequireRole(middleware.RoleAdminKeuangan))
			{
				adminGroup.GET("/students", tagihanHandler.ListStudents)
				adminGroup.GET("/invoices", tagihanHandler.ListAdminInvoices)
				adminGroup.POST("/invoices", tagihanHandler.CreateAdminInvoice)
			}

			// Endpoint khusus Mahasiswa (Diisolasi ketat per institusi_id & user_id dari klaim JWT)
			studentGroup := protected.Group("/student")
			studentGroup.Use(middleware.RequireRole(middleware.RoleMahasiswa))
			{
				studentGroup.GET("/invoices", tagihanHandler.ListStudentInvoices)
				studentGroup.GET("/invoices/:id", tagihanHandler.GetStudentInvoiceDetail)
			}
		}
	}

	srv := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("[SERVER] Sistem Manajemen Tagihan UKT Multi-Institusi berjalan pada port :%s", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Server gagal berjalan: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[SERVER] Mematikan server secara graceful...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("[FATAL] Server dipaksa berhenti: %v", err)
	}
	log.Println("[SERVER] Server berhasil berhenti.")
}
