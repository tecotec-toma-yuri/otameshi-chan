package router

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/otameshi/backend/internal/config"
	"github.com/otameshi/backend/internal/factory"
	"github.com/otameshi/backend/internal/handler/rest"
	"github.com/otameshi/backend/internal/handler/ws"
	appMiddleware "github.com/otameshi/backend/internal/middleware"
)

func New() chi.Router {
	wsHandler := ws.NewHandler(factory.NewChatDependencies())

	r := chi.NewRouter()

	// 1. グローバルミドルウェア (実行順序が重要)
	r.Use(middleware.RequestID)                 // 最優先: ログ追跡用のリクエストID生成
	r.Use(middleware.RealIP)                    // プロキシ・ロードバランサー配下のIP正しく取得
	r.Use(appMiddleware.Logger)                 // slogを使用した構造化アクセスログ
	r.Use(middleware.Recoverer)                 // ハンドラー内の panic から自動復旧
	r.Use(middleware.Timeout(60 * time.Second)) // リクエスト全体のタイムアウト設定

	// 2. CORS設定
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{config.Infra().CORSOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300, // Preflightリクエストのキャッシュ期間(秒)
	}))

	// 3. パブリックエンドポイント (未認証でOKなもの)
	r.Get("/health", rest.HandleHealth)

	// 4. WebSocketエンドポイント
	r.Handle("/ws", wsHandler)

	// 5. REST APIグループ
	r.Route("/api", func(r chi.Router) {
		// 未認証API
		r.Get("/config", rest.HandleConfigGet)

		// 認証保護が必要なAPIグループ (認証ミドルウェア導入時に有効化)
		r.Group(func(r chi.Router) {
			// r.Use(appMiddleware.Authenticate)

			r.Put("/config", rest.HandleConfigPut)
			r.Get("/models", rest.HandleModelsGet)
			r.Get("/tts/voices", rest.HandleTTSVoicesGet)
			r.Get("/history", rest.HandleHistoryList)
			r.Get("/history/{id}", rest.HandleHistoryGet)
		})
	})

	return r
}
