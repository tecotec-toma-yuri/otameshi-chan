# おためしちゃん

音声駆動型・文脈察知式商品紹介アシスタント。ユーザーの音声またはテキスト入力に対し、AIが商品推薦や会話応答を行い、音声で返答するシステム。

## アーキテクチャ

```
┌─────────────┐     WebSocket      ┌──────────────┐
│  Frontend   │◄──────────────────►│   Backend    │
│  (Nuxt 3)   │                    │   (Go 1.23)  │
└─────────────┘                    └──────┬───────┘
                                          │
                          ┌───────────────┼───────────────┐
                          │               │               │
                    ┌─────▼─────┐  ┌──────▼──────┐ ┌─────▼─────┐
                    │    STT    │  │     LLM     │ │    TTS    │
                    │ Whisper   │  │  Chat Comp. │ │Piper Plus │
                    │   .cpp    │  │  (Groq等)   │ │/ VOICEVOX │
                    └───────────┘  └─────────────┘ └───────────┘
```

## サービス構成

| サービス | ポート | 役割 |
|----------|--------|------|
| `frontend` | 3000 | Nuxt 3 SPA（音声入力UI、会話表示） |
| `backend` | 8080 | Go WebSocket APIサーバー |
| `tts` | 5000 | Piper Plus TTS（多言語対応） |
| `voicevox` | 50021 | VOICEVOX TTS（日本語高品質音声） |
| `stt` | 8888 | Whisper.cpp STT |

## セットアップ

### 前提条件

- Docker / Docker Compose

### 1. 環境変数の設定

```bash
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env
```

`backend/.env` を編集し、以下を設定:

```env
LLM_MODE=chat
OPENAI_API_KEY=your-api-key
OPENAI_BASE_URL=https://api.groq.com/openai/v1   # Groqの場合
```

### 2. 起動

```bash
docker compose up -d
```

ブラウザで http://localhost:3000 にアクセス。

## 主要機能

### 音声会話
- ブラウザマイクからの音声入力 → Whisper.cpp でSTT → LLM応答 → TTS音声再生
- テキスト入力にも対応
- バージイン（AI発話中の割り込み）対応

### 商品推薦
- LLMがFunction Calling（`recommend_product`）で商品カードを表示
- 設定画面から商品情報やシステムプロンプトをカスタマイズ可能

### 多言語TTS
- Piper TTS: 日本語・英語・中国語・スペイン語・フランス語・ポルトガル語対応（base_6langモデル）
- VOICEVOX: 日本語高品質音声
- STT検出言語に自動追従する`auto`モード

### 会話履歴
- セッション終了時に自動保存
- 履歴一覧・詳細表示
- 過去の会話からセッション再開

### 無音タイマー
- 60秒無音 → 確認メッセージ → さらに30秒無音 → 自動終了

## 設定

`backend/.env` の主要設定:

| 変数 | 説明 | デフォルト |
|------|------|-----------|
| `LLM_MODE` | LLMモード（`stub` / `chat`） | `stub` |
| `OPENAI_API_KEY` | LLM APIキー | - |
| `OPENAI_BASE_URL` | LLM APIエンドポイント | `https://api.openai.com/v1` |
| `TTS_ENGINE` | TTSエンジン（`piper` / `voicevox`） | `piper` |
| `STT_LANGUAGE` | STT認識言語 | `ja` |

Web UIの設定画面（http://localhost:3000/settings）からLLMモデル、TTSパラメータ、システムプロンプト、商品情報を動的に変更可能。

## ディレクトリ構成

```
├── backend/              # Go バックエンド
│   ├── cmd/server/       # エントリポイント
│   ├── internal/
│   │   ├── config/       # 設定管理
│   │   ├── handler/      # HTTP/WS ハンドラ
│   │   ├── history/      # 会話履歴保存
│   │   ├── openai/       # LLM クライアント
│   │   ├── protocol/     # WebSocket メッセージ定義
│   │   ├── session/      # セッション管理・状態遷移
│   │   ├── stt/          # 音声認識
│   │   ├── tts/          # 音声合成
│   │   └── textbuf/      # テキストバッファ（文単位分割）
│   └── data/             # 設定・履歴データ
├── frontend/             # Nuxt 3 フロントエンド
│   ├── pages/            # assistant, settings, history
│   ├── composables/      # useSession, useWebSocket, etc.
│   ├── components/       # UI コンポーネント
│   └── types/            # TypeScript 型定義
├── tts/                  # Piper Plus TTS サーバー
├── stt/                  # Whisper.cpp STT サーバー
└── docker-compose.yml
```
