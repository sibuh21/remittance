package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"remittance-service/internal/boa"
	"remittance-service/internal/cybersource"
	"remittance-service/internal/database"
	"remittance-service/internal/domain"
	"remittance-service/internal/handler"
	"remittance-service/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/viper"
)

func main() {
	// ─── Load Configuration ─────────────────────────────────────────────────
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// ─── Initialize Database (PostgreSQL) ───────────────────────────────────
	db, err := database.NewConnection(*cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	if err := db.InitializeSchema(); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	// ─── Initialize CyberSource Client (Inbound Collection) ─────────────────
	csClient, err := cybersource.NewClient(
		cfg.CyberSource.AccessKey,
		cfg.CyberSource.ProfileID,
		cfg.CyberSource.SecretKey,
		cfg.CyberSource.CheckoutURL,
	)
	if err != nil {
		log.Fatalf("Failed to create CyberSource client: %v", err)
	}

	// ─── Initialize Bank of Abyssinia Client (Outbound Payout) ──────────────
	boaClient, err := boa.NewClient(
		cfg.BoA.BaseURL,
		cfg.BoA.TokenURL,
		cfg.BoA.ClientID,
		cfg.BoA.ClientSecret,
		cfg.BoA.RefreshToken,
		cfg.BoA.APIKey,
	)
	if err != nil {
		log.Fatalf("Failed to create BoA client: %v", err)
	}

	// ─── Initialize Services ────────────────────────────────────────────────
	collectionSvc := service.NewCollectionService(csClient)
	payoutSvc := service.NewPayoutService(boaClient)
	remittanceSvc := service.NewRemittanceService(collectionSvc, payoutSvc, db)

	// ─── Initialize Handlers ────────────────────────────────────────────────
	collectionHandler := handler.NewCollectionHandler(collectionSvc, remittanceSvc)
	payoutHandler := handler.NewPayoutHandler(payoutSvc)
	remittanceHandler := handler.NewRemittanceHandler(remittanceSvc)

	// ─── Setup Echo Server ──────────────────────────────────────────────────
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.CORS.AllowedOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderXRequestID,
			echo.HeaderAuthorization,
		},
	}))

	// ─── API Routes ─────────────────────────────────────────────────────────

	api := e.Group("/api")

	// === Remittance (End-to-End Flow) ===
	// POST /api/remittance             - Initiate a remittance (validate + checkout fields)
	// POST /api/remittance/payout      - Manually trigger payout for a collected remittance
	// GET  /api/remittance/status/:id   - Get transaction status
	api.POST("/remittance", remittanceHandler.InitiateRemittance)
	api.POST("/remittance/payout", remittanceHandler.TriggerPayout)
	api.GET("/remittance/status/:id", remittanceHandler.GetStatus)
	api.GET("/remittance/sender/:email", remittanceHandler.ListSenderRemittances)
	api.GET("/remittance/receiver/:phone", remittanceHandler.ListReceiverRemittances)

	// === Collection (CyberSource Inbound) ===
	// POST /api/checkout  - Get signed fields for CyberSource hosted checkout
	// POST /api/response  - CyberSource return URL (customer redirect)
	// POST /api/webhook   - CyberSource silent POST notification
	api.POST("/checkout", collectionHandler.GenerateSignedFields)
	api.POST("/response", collectionHandler.HandleResponse)
	api.POST("/webhook", collectionHandler.HandleWebhook)

	// === Payout (Bank of Abyssinia Outbound) ===
	// POST /api/payout/validate      - Validate beneficiary account/wallet
	// GET  /api/payout/rate/:currency - Get exchange rate
	// GET  /api/payout/banks          - Get available banks for other-bank transfer
	// GET  /api/payout/balance        - Get settlement account balance
	// GET  /api/payout/status/:id     - Check payout transaction status
	payout := api.Group("/payout")
	payout.POST("/validate", payoutHandler.ValidateBeneficiary)
	payout.GET("/rate/:currency", payoutHandler.GetExchangeRate)
	payout.GET("/banks", payoutHandler.GetBanks)
	payout.GET("/balance", payoutHandler.GetBalance)
	payout.GET("/status/:id", payoutHandler.CheckTransactionStatus)

	// ─── Checkout Result Pages ──────────────────────────────────────────────
	e.GET("/checkout/success", func(c echo.Context) error {
		return c.File("frontend/checkout/success.html")
	})
	e.GET("/checkout/declined", func(c echo.Context) error {
		return c.File("frontend/checkout/declined.html")
	})
	e.GET("/checkout/error", func(c echo.Context) error {
		return c.File("frontend/checkout/error.html")
	})
	e.GET("/checkout/cancelled", func(c echo.Context) error {
		return c.File("frontend/checkout/cancelled.html")
	})
	e.GET("/checkout/review", func(c echo.Context) error {
		return c.File("frontend/checkout/review.html")
	})

	// ─── Serve Frontend Static Files ────────────────────────────────────────
	e.Static("/", "frontend")

	// ─── Health Check ───────────────────────────────────────────────────────
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "remittance-service",
		})
	})

	// ─── Start Server ───────────────────────────────────────────────────────
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("═══════════════════════════════════════════════════════════════")
	log.Printf("  Remittance Service starting on %s", addr)
	log.Printf("  CyberSource Checkout: %s", cfg.CyberSource.CheckoutURL)
	log.Printf("  BoA Base URL:         %s", cfg.BoA.BaseURL)
	log.Printf("  CORS Allowed Origins: %s", strings.Join(cfg.CORS.AllowedOrigins, ", "))
	log.Printf("═══════════════════════════════════════════════════════════════")

	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

func loadConfig() (*domain.Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg domain.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
