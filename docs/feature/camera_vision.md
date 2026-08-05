# カメラ連携ビジョン機能 実装仕様書

## 概要

フロントエンドのカメラでユーザーの映像を定期キャプチャし、LLM（Gemini）のビジョン機能に渡すことで、ユーザーの見た目や周囲の状況に言及した会話を実現する。

併せて、LLMプロバイダをGroqからGemini（無料枠）に切り替える。

## 現在の構成

| コンポーネント | 現状 |
|---|---|
| LLM | Groq API（OpenAI互換、`llama-3.1-8b-instant`） |
| STT | Whisper（`STT_URL`で外部指定） |
| TTS | Piper / VOICEVOX（`TTS_ENGINE`で切替） |
| プロトコル | WebSocket（クライアント↔バックエンド） |
| 画像処理 | なし |

## 変更後の構成

| コンポーネント | 変更後 |
|---|---|
| LLM | **Gemini API**（OpenAI互換エンドポイント、`gemini-3.5-flash`） |
| STT | Whisper（変更なし） |
| TTS | Piper / VOICEVOX（変更なし） |
| 画像処理 | **新規追加** |

---

## 1. LLMプロバイダ切替（Groq → Gemini）

### 変更箇所

`.env` の環境変数のみ。コード変更なし。

```env
# 変更前（Groq）
OPENAI_BASE_URL=https://api.groq.com/openai/v1
OPENAI_API_KEY=gsk_xxxxx
OPENAI_MODEL=llama-3.1-8b-instant

# 変更後（Gemini）
OPENAI_BASE_URL=https://generativelanguage.googleapis.com/v1beta/openai/
OPENAI_API_KEY=（Google AI Studio で発行した Gemini API キー）
OPENAI_MODEL=gemini-3.5-flash
```

### Gemini 無料枠の制約

| 項目 | 内容 |
|---|---|
| 料金 | 入力・出力トークンともに無料 |
| レート制限 | RPM/TPM 制限あり（Flash系は比較的緩い） |
| プライバシー | 無料枠ではプロダクト改善にコンテンツが使用される |
| 互換性 | OpenAI互換エンドポイント経由でストリーミング・Function Calling・画像入力すべて対応 |

### 既存機能への影響

- `ChatCompletionClient`（`backend/internal/openai/chat.go`）: `baseURL`・`apiKey`・`model` を環境変数から読み込む既存ロジックがそのまま動作
- SSEストリーミング: Gemini互換エンドポイントがサポート済み
- Function Calling（`recommend_product`、`end_conversation`）: Gemini互換エンドポイントがサポート済み
- ガードレール（`backend/internal/guardrail/llm.go`）: 同じ`OPENAI_BASE_URL`/`OPENAI_API_KEY`を使用しているため、Geminiに自動で切り替わる

---

## 2. カメラ連携ビジョン機能

### 2.1 動作モード

**定期キャプチャ方式**を採用する。

- 一定間隔（デフォルト10秒）でカメラからフレームをキャプチャ
- バックエンドに送信し、最新フレームを保持
- 次回のLLMリクエスト時にマルチモーダル入力として画像を添付
- AIが自然な流れでユーザーの外見や状況に言及する

### 2.2 フロントエンド実装

#### 新規ファイル: `frontend/composables/useCamera.ts`

```typescript
export function useCamera() {
  const isActive = ref(false)
  const stream = ref<MediaStream | null>(null)
  const videoEl = ref<HTMLVideoElement | null>(null)
  const captureInterval = ref(10000) // 10秒
  const isSupported = ref(true)

  let intervalId: ReturnType<typeof setInterval> | null = null

  async function startCamera(): Promise<void>
  function stopCamera(): void
  function captureFrame(): string | null  // Base64 JPEG を返す
  function setCaptureInterval(ms: number): void

  return {
    isActive: readonly(isActive),
    isSupported: readonly(isSupported),
    stream: readonly(stream),
    videoEl,
    startCamera,
    stopCamera,
    captureFrame,
    setCaptureInterval,
  }
}
```

**キャプチャ処理の詳細:**

1. `getUserMedia({ video: { width: 512, height: 512, facingMode: 'user' } })` でストリーム取得
2. `<video>` 要素にストリームをバインド
3. タイマーで定期的に `<canvas>` に描画 → `canvas.toDataURL('image/jpeg', 0.7)` でBase64化
4. WebSocket経由でバックエンドに送信

**画像サイズ:**

- 解像度: 512×512（トークン消費抑制のため）
- フォーマット: JPEG（quality: 0.7）
- 1フレームあたり約30〜50KB

#### UI変更: `frontend/pages/assistant.vue`

- ヘッダーまたは入力エリアにカメラON/OFFトグルボタンを追加
- カメラON時に画面の隅に小型プレビュー表示（PiP風、100×100px程度）
- カメラ権限拒否時はボタンを disabled にしエラー表示

```
ヘッダー: おためしちゃん [StatusIndicator] [復元] [新規会話] ... [📷] [音量] [履歴] [設定] [接続/切断]
```

#### カメラ送信フロー

```
[タイマー発火] → captureFrame() → Base64 JPEG
    → WebSocket送信: { type: "image_frame", image: "<base64>" }
    → バックエンド: 最新フレームとして保持（1枚のみ）
```

### 2.3 WebSocket プロトコル追加

#### Client → Server（新規メッセージタイプ）

```json
{
  "type": "image_frame",
  "image": "<Base64エンコードされたJPEG画像>"
}
```

#### 対応ファイル

- `frontend/types/protocol.ts`: `ClientMessage` に `image_frame` タイプ追加
- `backend/internal/protocol/messages.go`: `ImageFrame` 構造体追加

### 2.4 バックエンド実装

#### ChatMessage のマルチパート対応

**変更ファイル:** `backend/internal/openai/chat.go`

現在の `ChatMessage.Content` は `string` 型だが、画像付きメッセージではマルチパート（配列）に対応する必要がある。

```go
// 変更前
type ChatMessage struct {
    Role    string `json:"role"`
    Content string `json:"content,omitempty"`
    // ...
}

// 変更後
type ChatMessage struct {
    Role    string      `json:"role"`
    Content interface{} `json:"content,omitempty"` // string or []ContentPart
    // ...
}

type ContentPart struct {
    Type     string    `json:"type"`
    Text     string    `json:"text,omitempty"`
    ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
    URL    string `json:"url"`
    Detail string `json:"detail,omitempty"` // "low" でトークン節約
}
```

#### 最新フレームの保持

**変更ファイル:** `backend/internal/session/manager.go`

```go
type Manager struct {
    // 既存フィールド...
    latestImageFrame string // 最新のカメラフレーム（Base64 JPEG）
    mu               sync.Mutex
}
```

- `image_frame` メッセージ受信時に `latestImageFrame` を上書き（常に最新1枚のみ）
- 会話履歴には画像を含めない（トークン節約）

#### LLMリクエストへの画像添付

**変更ファイル:** `backend/internal/openai/chat.go`

`Send()` メソッドでLLMにリクエストを送る際、最新フレームがあればユーザーメッセージをマルチパートに変換する。

```go
func (c *ChatCompletionClient) buildUserMessage(text string, imageBase64 string) ChatMessage {
    if imageBase64 == "" {
        return ChatMessage{Role: "user", Content: text}
    }
    return ChatMessage{
        Role: "user",
        Content: []ContentPart{
            {Type: "text", Text: text},
            {Type: "image_url", ImageURL: &ImageURL{
                URL:    "data:image/jpeg;base64," + imageBase64,
                Detail: "low",
            }},
        },
    }
}
```

画像添付後は `latestImageFrame` をクリアし、同じフレームが繰り返し送信されることを防ぐ。

#### システムプロンプト追加

**変更ファイル:** `backend/internal/config/config.go`（デフォルト設定）

既存のシステムプロンプトに以下を追記:

```
## カメラ連携
- カメラ映像が送られた場合、ユーザーの外見や周囲の状況に自然に言及してください
- 「赤い服がお似合いですね」「眼鏡をかけてるんですね」など、さりげなく触れる程度に
- 外見への言及は会話の流れの中で自然に行い、毎回言及する必要はありません
- ネガティブな外見への言及は避けてください
```

### 2.5 カメラ権限とプライバシー

| 項目 | 対応 |
|---|---|
| ブラウザ権限 | `getUserMedia` 呼び出し時にブラウザがユーザーに許可を求める |
| ON/OFF制御 | カメラトグルでいつでもON/OFF可能。OFF時はストリーム停止・フレーム送信停止 |
| プレビュー表示 | カメラON時に小型プレビューを表示し、撮影中であることを明示 |
| データ保持 | バックエンドは最新1フレームのみ保持。会話履歴やDBには画像を保存しない |
| Gemini無料枠 | プロダクト改善にコンテンツが使用される旨をユーザーに表示する |

---

## 3. 実装ステップ

### Phase 1: LLM切替

1. `.env` を Gemini API 用に変更
2. 動作確認（既存の音声会話がGeminiで動作するか）

### Phase 2: バックエンド画像対応

3. `ChatMessage.Content` をマルチパート対応に変更（`chat.go`）
4. `protocol/messages.go` に `ImageFrame` メッセージタイプ追加
5. `session/manager.go` に `latestImageFrame` フィールド追加、`image_frame` メッセージハンドリング
6. LLMリクエスト時の画像添付ロジック実装
7. システムプロンプトにカメラ連携の指示を追記

### Phase 3: フロントエンド

8. `composables/useCamera.ts` 作成（カメラ取得・キャプチャ・送信）
9. `types/protocol.ts` に `image_frame` タイプ追加
10. `pages/assistant.vue` にカメラトグル・プレビューUI追加
11. WebSocket送信処理に `image_frame` 送信を追加

### Phase 4: 結合テスト

12. カメラON → 定期キャプチャ → Geminiが外見に言及する会話の動作確認
13. カメラOFF時は従来通りのテキスト/音声のみの会話であることを確認
14. エッジケース確認（カメラ権限拒否、ブラウザ非対応、接続中のON/OFF切替）

---

## 4. ファイル変更一覧

| ファイル | 変更種別 | 内容 |
|---|---|---|
| `backend/.env` | 変更 | Gemini API の接続情報に変更 |
| `backend/internal/openai/chat.go` | 変更 | `ChatMessage.Content` をマルチパート対応、画像添付ロジック追加 |
| `backend/internal/protocol/messages.go` | 変更 | `ImageFrame` メッセージタイプ追加 |
| `backend/internal/session/manager.go` | 変更 | `latestImageFrame` 保持、`image_frame` ハンドリング |
| `backend/internal/config/config.go` | 変更 | システムプロンプトにカメラ連携指示を追記 |
| `frontend/composables/useCamera.ts` | **新規** | カメラ取得・キャプチャ・送信ロジック |
| `frontend/types/protocol.ts` | 変更 | `image_frame` クライアントメッセージ追加 |
| `frontend/composables/useWebSocket.ts` | 変更 | `sendImageFrame` メソッド追加 |
| `frontend/pages/assistant.vue` | 変更 | カメラトグルボタン・プレビューUI追加 |

---

## 5. 帯域・コスト見積もり

| 項目 | 値 |
|---|---|
| 1フレームサイズ | 約30〜50KB（512×512 JPEG, quality 0.7） |
| 送信頻度 | 10秒に1回 |
| 1分あたり通信量 | 約180〜300KB |
| 1分あたりAPIリクエスト増 | 0回（画像は次回のユーザー発話時にのみLLMに送信） |
| 画像トークン（`detail: low`） | 約85トークン/フレーム |
| Gemini無料枠での運用 | 問題なし |
