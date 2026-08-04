# おためしちゃん 改訂仕様書 v2.0

## 1. システム概要

音声駆動型・文脈察知式商品紹介アシスタント。ユーザーの音声またはテキスト入力に対し、AIが商品推薦や会話応答を行い、音声で返答するシステム。

### 1.1 変更点サマリ（v1 → v2）

| 項目 | v1 | v2 |
|------|----|----|
| LLM API | OpenAI Realtime API のみ | Realtime API / Chat Completion API 切替対応 |
| STT | Web Speech API（ブラウザ依存） | Whisper.cpp（ローカル、Dockerコンテナ） |
| TTS | Piper Plus（つくよみちゃん） | 変更なし |
| API切替 | - | 環境変数 `LLM_MODE` で制御 |
| 入力モード | テキスト / マイク切替 | 変更なし |

---

## 2. アーキテクチャ

```
┌─────────────┐     WebSocket      ┌──────────────┐
│  Frontend   │◄──────────────────►│   Backend    │
│  (Nuxt 3)   │                    │   (Go)       │
└─────────────┘                    └──────┬───────┘
                                          │
                          ┌───────────────┼───────────────┐
                          │               │               │
                    ┌─────▼─────┐  ┌──────▼──────┐ ┌─────▼─────┐
                    │  STT      │  │  LLM        │ │  TTS      │
                    │ Whisper   │  │ Realtime or │ │ Piper Plus│
                    │  .cpp     │  │ Chat Compl. │ │           │
                    └───────────┘  └─────────────┘ └───────────┘
```

### 2.1 Dockerサービス構成

| サービス | ポート | 役割 |
|----------|--------|------|
| `frontend` | 3000 | Nuxt 3 SPA |
| `backend` | 8080 | Go WebSocket API サーバー |
| `tts` | 5000 | Piper Plus TTS（つくよみちゃん） |
| `stt` | 8888 | Whisper.cpp HTTP サーバー |

---

## 3. STT（音声→テキスト変換）

### 3.1 エンジン: Whisper.cpp

- **実装**: whisper.cpp の HTTP サーバーモード（`server` バイナリ）
- **モデル**: `ggml-small`（日本語精度と速度のバランス）
- **言語**: `ja`（日本語固定）
- **Docker**: 専用コンテナでビルド・起動
- **推論**: CPU推論（GPUなし環境対応）

### 3.2 STT API仕様

```
POST /inference
Content-Type: multipart/form-data

パラメータ:
  - file: WAVファイル（16kHz, mono, 16-bit PCM）
  - language: "ja"
  - response_format: "json"

レスポンス:
  {
    "text": "認識されたテキスト"
  }
```

### 3.3 音声入力フロー

```
[ブラウザ マイク] → PCM音声チャンク（250ms間隔）
    → WebSocket → Backend
    → クライアント側VAD（無音検知: 1.5秒）
    → 発話区間の音声をWAV化
    → Whisper.cpp HTTP API → テキスト
    → LLM処理へ
```

#### VAD（発話区間検出）仕様

- **方式**: フロントエンド側でRMS音量ベースのVAD
- **発話開始**: RMS値が閾値（0.01）を超えた時点
- **発話終了**: RMS値が閾値以下の状態が1.5秒継続
- **最小発話長**: 300ms未満の発話は無視（ノイズ除外）
- **最大発話長**: 30秒で強制区切り
- 発話終了検知後、蓄積した音声データをバックエンドに送信
- バックエンド側でWAV形式に変換し、Whisper.cpp に送信

---

## 4. LLM API

### 4.1 モード切替

環境変数 `LLM_MODE` で制御:

| 値 | 動作 |
|----|------|
| `realtime` | OpenAI Realtime API 使用 |
| `chat` | OpenAI Chat Completion API 使用 |
| `stub`（デフォルト） | キーワードベースのスタブ応答 |

切替は `docker-compose.yml` の `backend` サービスで設定:

```yaml
environment:
  - LLM_MODE=stub        # stub / chat / realtime
  - OPENAI_API_KEY=sk-...  # chat / realtime 時に必要
  - OPENAI_MODEL=gpt-4o   # chat モード時のモデル
```

### 4.2 Chat Completion API モード

#### リクエストフロー

```
ユーザー発話 → STT → テキスト
  → Chat Completion API（ストリーミング）
  → テキスト応答（text_delta / text_done）
  → TTS → 音声
```

#### 会話履歴管理

- セッション中の全メッセージ履歴を保持
- 各リクエスト時に全履歴を `messages` パラメータとして送信
- システムプロンプトで商品紹介アシスタントとしての役割を定義

#### システムプロンプト

```
あなたは商品紹介アシスタント「おためしちゃん」です。
お客様と自然な日本語で会話しながら、以下の商品を紹介してください。

【取扱商品】
- h001: ハイチュウ ＜グレープ＞（¥140）- ジューシーな果汁感あふれる定番フレーバー
- h002: ハイチュウ ＜ストロベリー＞（¥140）- いちごの華やかな香りと甘酸っぱさ
- h003: ハイチュウ ＜グリーンアップル＞（¥140）- 爽やかな香りとスッキリとした酸味

お客様が商品に興味を示したら recommend_product 関数を呼び出してください。
会話を終了する際は end_conversation 関数を呼び出してください。
```

#### Function Calling 定義

Realtime API / Chat Completion API 共通:

```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "recommend_product",
        "description": "お客様に商品を紹介する",
        "parameters": {
          "type": "object",
          "properties": {
            "product_ids": {
              "type": "array",
              "items": { "type": "string" },
              "description": "紹介する商品IDのリスト"
            },
            "introduction_speech": {
              "type": "string",
              "description": "商品紹介時の発話テキスト"
            }
          },
          "required": ["product_ids", "introduction_speech"]
        }
      }
    },
    {
      "type": "function",
      "function": {
        "name": "end_conversation",
        "description": "会話を終了する",
        "parameters": {
          "type": "object",
          "properties": {
            "closing_speech": {
              "type": "string",
              "description": "終了時の挨拶テキスト"
            }
          },
          "required": ["closing_speech"]
        }
      }
    }
  ]
}
```

#### ストリーミング応答

- `stream: true` でストリーミング応答を取得
- テキストチャンクを `text_delta` としてフロントエンドに即時配信
- ストリーム完了時に `text_done` + TTS音声を配信

### 4.3 Realtime API モード

- v1仕様書の通り（WebSocket接続、音声入出力）
- 音声の直接入出力をサポート（STT/TTSはAPI側で処理）
- 現時点ではスタブ実装のみ

### 4.4 スタブモード

- 現行のキーワードベース応答を維持
- 「おすすめ」→ 全商品推薦、「ありがとう」→ 終了、等

---

## 5. TTS（テキスト→音声変換）

### 5.1 エンジン: Piper Plus（変更なし）

- **モデル**: つくよみちゃん（tsukuyomi-chan-6lang-fp16）
- **出力**: WAV（16kHz, mono, 16-bit PCM）
- **話速**: `length_scale=1.3`（環境変数で調整可能）

### 5.2 API仕様（変更なし）

```
POST /synthesize
Content-Type: application/json

{"text": "読み上げるテキスト", "length_scale": 1.3}

レスポンス: audio/wav バイナリ
```

---

## 6. バックエンド内部構成

### 6.1 LLMクライアントインターフェース

```go
type LLMClient interface {
    SendUserMessage(ctx context.Context, text string) (<-chan StreamEvent, error)
    Close()
}
```

実装:
- `StubClient` — キーワードベース（既存）
- `ChatCompletionClient` — OpenAI Chat Completion API（新規）
- `RealtimeClient` — OpenAI Realtime API（既存、将来実装）

### 6.2 STTクライアント

```go
type STTService interface {
    Transcribe(ctx context.Context, audioData []byte) (string, error)
}
```

実装:
- `WhisperSTT` — Whisper.cpp HTTP API クライアント（新規）
- `StubSTT` — 何もしない（テキスト入力モード用）

### 6.3 メッセージフロー（Chat Completion モード）

```
1. Frontend → audio_chunk (PCM base64) → Backend
2. Backend: PCM蓄積 → VAD発話終了検知 → WAV化
3. Backend → Whisper.cpp: POST /inference → テキスト
4. Backend → Chat Completion API: messages[] + tools → ストリーミング応答
5. 応答テキスト → text_delta → Frontend
6. 応答完了 → TTS → text_done + audio_chunk → Frontend
7. Function Call発火時 → 商品推薦 or セッション終了
```

### 6.4 メッセージフロー（テキスト入力モード）

```
1. Frontend → audio_chunk { text: "入力テキスト" } → Backend
2. Backend → Chat Completion API（STTスキップ）
3. 以降同上
```

---

## 7. フロントエンド

### 7.1 画面構成（変更なし）

- ヘッダー: タイトル + 接続バッジ + 音量スライダー + ミュート + 接続ボタン
- ステータスバー: 状態表示 + テキスト/マイク切替トグル
- メインエリア: 会話タイムライン + 商品カード
- フッター: テキスト入力 or マイクボタン（モードにより切替）

### 7.2 マイクモード動作

- マイクON → ブラウザ `getUserMedia` で音声取得
- RMSベースVADで発話区間検出
- 発話終了検知 → 音声データをWebSocket経由でバックエンドに送信
- バックエンドがWhisper.cppでSTT → LLM処理
- 認識中テキスト（interim）はバックエンドからリアルタイム配信しない
  （Whisper.cppはバッチ処理のため、Web Speech APIのようなinterim結果はなし）

---

## 8. 環境変数一覧

| 変数名 | サービス | デフォルト | 説明 |
|--------|----------|-----------|------|
| `LLM_MODE` | backend | `stub` | LLMモード（`stub` / `chat` / `realtime`） |
| `OPENAI_API_KEY` | backend | - | OpenAI APIキー |
| `OPENAI_MODEL` | backend | `gpt-4o` | Chat Completionモデル |
| `TTS_URL` | backend | - | TTS サーバーURL |
| `STT_URL` | backend | - | STT サーバーURL |
| `TTS_LENGTH_SCALE` | tts | `1.3` | TTS話速（大きい=遅い） |
| `TTS_NOISE_SCALE` | tts | `0.667` | TTS声の変動 |
| `TTS_NOISE_W` | tts | `0.8` | TTS音素長の変動 |
| `WHISPER_MODEL` | stt | `small` | Whisperモデルサイズ |
| `NUXT_PUBLIC_WS_URL` | frontend | `ws://localhost:8080/ws` | WebSocket URL |

---

## 9. docker-compose.yml（想定構成）

```yaml
services:
  backend:
    build: ./backend
    ports: ["8080:8080"]
    environment:
      - LLM_MODE=stub
      - TTS_URL=http://tts:5000
      - STT_URL=http://stt:8888
    depends_on:
      tts: { condition: service_healthy }
      stt: { condition: service_healthy }

  frontend:
    build: ./frontend
    ports: ["3000:3000"]
    environment:
      - NUXT_PUBLIC_WS_URL=ws://localhost:8080/ws
    depends_on: [backend]

  tts:
    build: ./tts
    ports: ["5000:5000"]
    environment:
      - TTS_LENGTH_SCALE=1.3

  stt:
    build: ./stt
    ports: ["8888:8888"]
    environment:
      - WHISPER_MODEL=small
```

---

## 10. 実装優先順位

### Phase 1: STTコンテナ（Whisper.cpp）
- Whisper.cpp Docker イメージ作成
- HTTP API動作確認

### Phase 2: バックエンド STT 統合
- STTService インターフェース + WhisperSTT 実装
- 音声チャンク蓄積 → VAD → Whisper送信フロー
- マイクモード動作確認

### Phase 3: Chat Completion API 対応
- ChatCompletionClient 実装（ストリーミング）
- 会話履歴管理
- Function Calling 処理
- LLM_MODE による切替

### Phase 4: 統合テスト
- stub / chat モードの動作確認
- テキスト入力 + マイク入力の両方で検証
