# 音声駆動型・文脈察知式商品紹介アシスタントシステム 詳細設計書 v6.0

## 1. 全体機能要件

### 1.1 システム概要

本システムは、Nuxt 3（フロントエンド）と Go（APIサーバー）間を WebSocket で常時接続し、**OpenAI互換 Chat Completions API（SSEストリーミング）** を用いてリアルタイムのテキスト対話を実現するシステムである。

AIはシステムプロンプトとして渡された固定商品カタログを常時把握し、雑談やヒアリングを通じてユーザーのニーズを察知したタイミングで、**Function Calling（`recommend_product`）を用いて「おすすめ商品」と「ユーザーの文脈に合わせた紹介文言（`introduction_speech`）」を出力**する。

レコメンドは**常に「1回の提案行為」として行う**。1回の提案で紹介するのは単一の商品が基本だが、ユーザーが特定のカテゴリやシリーズに興味を示した場合には、**関連する複数商品をまとめて1回のレコメンドとして紹介**できる（例：「ハイチュウにはグレープ、ストロベリー、グリーンアップルがありますよ！」）。この場合もあくまで「1回の提案」であり、個別のセット販売商品ではない。

また、設定により**指定した複数商品を順番に連続レコメンド**するモードや、**レコメンド後の会話挙動**（通常会話への復帰 or 他商品への関心確認）を切り替えることができる。

ユーザー体験を高めるため**割り込み会話（Barge-in）**および**多角的な会話終了判定・タイムアウト制御**に対応する。AIの発話中にユーザーが割り込んで喋り始めた場合は即座にAI生成をキャンセルし、フロント側の音声再生を停止する。対話完了や放置時には適切なセッション終了処理を行う。

音声出力には外部TTSサービス（**Piper Plus** または **VOICEVOX**）を使用する。Goサーバー側でLLMから受け取ったテキストを文単位にバッファリングし、TTSで音声化してWebSocket経由でブラウザへストリーミング配信する。Function Calling 発火時は `introduction_speech` を直接外部TTSで音声化して配信し、商品カードとともに最速で返す。

音声入力には **faster-whisper**（ローカル）または **Groq API**（クラウド）によるSTTを使用する。設定画面でエンジンを切り替え可能。フロントエンドではブラウザマイクからの音声を **RNNoise（WebAssembly）** でノイズ抑制し、**AudioWorklet API** で音声キャプチャした後、**Opus/WebM形式にエンコード**してサーバーへ送信する。

#### 1.1.1 LLM統合方式

| 項目 | 仕様 |
|:---|:---|
| **API** | OpenAI互換 Chat Completions API |
| **エンドポイント** | `POST {OPENAI_BASE_URL}/chat/completions` |
| **ストリーミング** | SSE（Server-Sent Events）。`data:` 行を逐次パースし `data: [DONE]` で終了 |
| **デフォルトプロバイダ** | Groq（`https://api.groq.com/openai/v1`） |
| **モデル** | 設定画面から動的選択（デフォルト: `gpt-4o-mini`） |
| **Function Calling** | 対応。ツール非対応モデルではHTTP 400レスポンス検知時にツールなしで自動リトライし、以降ツール送信を無効化 |
| **SSEエラー検知** | ストリームデータ内の `"error"` キーを検出し、`error` イベントとして上位に伝搬 |

#### 1.1.2 テキストバッファリング戦略

LLMからのテキストストリームに対し、Go サーバー側では以下のルールでバッファリングしてTTSに送信する：

| ルール | 条件 |
|:---|:---|
| **文境界フラッシュ** | 「。」「、」「！」「？」「…」で区切られた日本語テキストを1ユニットとして送信 |
| **文字数オーバーフロー** | 100文字（rune）以上蓄積された場合に強制送信 |
| **タイムアウト** | 最後のテキスト受信から1秒経過で残りバッファを強制送信 |

#### 1.1.3 外部TTSプロバイダ

| エンジン | 概要 | 対応言語 |
|:---|:---|:---|
| **Piper Plus** | 軽量ニューラルTTS。複数モデル・話者対応 | 日本語・英語・中国語・スペイン語・フランス語・ポルトガル語 |
| **VOICEVOX** | 日本語高品質音声合成エンジン | 日本語 |

---

### 1.2 レコメンドの基本方針

#### 1.2.1 「1回の提案」の定義

本システムにおけるレコメンドは、常に**1回の提案行為**として実行される。1回の `recommend_product` FC 発火に対して、フロントエンドへの商品カード配信と TTS 音声配信を1セットとして行う。

| パターン | 説明 | 例 |
|:---|:---|:---|
| **単一商品の紹介** | ユーザーのニーズに最も合う1商品を紹介 | 「酸っぱいのがお好きでしたら、グリーンアップル味がおすすめですよ！」 |
| **関連商品のまとめ紹介** | ユーザーが特定のカテゴリ・シリーズに興味を示した場合、関連する複数商品をまとめて紹介 | 「ハイチュウにはグレープ、ストロベリー、グリーンアップルの3種類がありますよ！」 |

#### 1.2.2 まとめ紹介とセット販売の違い

まとめ紹介は**AIが文脈に応じて関連商品を並べて紹介する行為**であり、カタログ上のセット販売商品とは異なる。各商品はカタログ上で独立した単品として存在する。商品の選択・購入機能は本システムのスコープ外であり、音声対話による商品紹介と情報提供を目的とする。

---

### 1.3 セッション設定

#### 1.3.1 レコメンドモード (`recommendation_mode`)

| モード値 | 説明 | 動作 |
|:---|:---|:---|
| **`ai_driven`** (デフォルト) | AIが会話文脈から判断して自発的にレコメンド | AIの FC 発火に任せる |
| **`sequential`** | 指定した商品リストを順番に連続レコメンド | Go サーバーが商品キューを管理し、1つのレコメンド完了後に次の商品をレコメンドする |

#### 1.3.2 順次レコメンドモード (`sequential`) の詳細

`sequential` モード有効時、Go サーバーはセッション設定で渡された商品IDリスト（またはまとめ紹介用の商品IDグループ）を内部キュー（`seqIndex`）として保持する。

```json
{
  "recommendation_mode": "sequential",
  "sequential_items": [
    { "product_ids": ["h001"] },
    { "product_ids": ["h002"] },
    { "product_ids": ["h001", "h002", "h003"] }
  ],
  "sequential_interval_sec": 5
}
```

#### 1.3.3 レコメンド後挙動 (`post_recommendation_behavior`)

| 設定値 | 説明 | AIの発話イメージ |
|:---|:---|:---|
| **`return_to_conversation`** (デフォルト) | 通常の雑談・対話に戻る | — |
| **`ask_interest`** | 他に気になる商品がないか明示的に確認 | 「ご紹介した商品はいかがですか？気になるものはありましたか？」 |

#### 1.3.4 セッション設定の全体構造

```json
{
  "recommendation_mode": "ai_driven | sequential",
  "sequential_items": [
    { "product_ids": ["h001"] },
    { "product_ids": ["h001", "h002", "h003"] }
  ],
  "sequential_interval_sec": 5,
  "post_recommendation_behavior": "return_to_conversation | ask_interest"
}
```

---

### 1.4 要求事項に紐づく機能要件

| ID | 分類 | 機能要件名 | 詳細説明 | 実装状態 |
|:---|:---|:---|:---|:---|
| **FR-01** | 対話制御 | リアルタイムテキスト対話 & 外部TTS | Chat Completions API（SSE）からテキストストリームを受け取り、文単位バッファリング後にPiper/VOICEVOXで音声化・配信 | ✅ 実装済 |
| **FR-02** | 状態察知 | 興味・ニーズの文脈察知 | AIはシステムプロンプトの指示に基づき、ユーザーが商品に興味を持ち始めたタイミングを自発的に判定 | ✅ 実装済 |
| **FR-03** | カタログ制御 | 商品レコメンド (FC) | `recommend_product` Function Calling で商品ID（単一または複数）と `introduction_speech` を受け取り、商品カードと音声を配信 | ✅ 実装済 |
| **FR-03a** | カタログ制御 | 関連商品のまとめ紹介 | `product_ids` 配列で複数商品をまとめて1回の提案として紹介可能 | ✅ 実装済 |
| **FR-03b** | カタログ制御 | 順次レコメンドモード | 指定商品リストを順番にレコメンド。`seqIndex` でキュー位置を管理 | ✅ 実装済 |
| **FR-03c** | 対話制御 | レコメンド後挙動設定 | `return_to_conversation` または `ask_interest` で切替可能 | ✅ 実装済 |
| **FR-04** | 最適化 | FC発火時の二重発話防止 | FC発火時は `introduction_speech` を直接TTSで音声化。テキストバッファをリセットし重複を防止 | ✅ 実装済 |
| **FR-05** | 割り込み制御 | 割り込み会話 (Barge-in) 対応 | AI発話中にユーザー入力を検知した場合、LLM/TTSパイプラインをキャンセルし `clear_audio_buffer` を送信 | ✅ 実装済 |
| **FR-06** | 会話管理 | 会話終了判定 & タイムアウト | `end_conversation` FC、手動切断、2段階無音タイマー（60秒+30秒）による切断制御 | ✅ 実装済 |
| **FR-07** | 安全性 | 非同期ガードレール監視 | ユーザー発話テキストおよびAI生成テキストをLLMベースのモデレーションで非同期チェック。違反時は `content_violation` イベントを配信 | ✅ 実装済 |

---

### 1.5 Function Calling 定義

#### 1.5.1 recommend_product

| パラメータ | 型 | 必須 | 説明 |
|:---|:---|:---|:---|
| `product_ids` | `string[]` | ✅ | 紹介する商品IDのリスト。設定済み商品IDから選択（`enum` で制限） |
| `introduction_speech` | `string` | ✅ | 商品を紹介するトーク文言 |

**発火条件**（`ai_driven` モード時）:
1. ユーザーが商品の品番・名前を明示的に問い合わせた場合
2. ユーザーが好みや用途を述べ、該当商品が存在する場合
3. ユーザーが直接的におすすめを求めた場合
4. 前回の提案に対してユーザーが否定・別案を求めた場合

**発火禁止条件**: 商品と無関係な雑談中 / カタログに該当商品なし

#### 1.5.2 end_conversation

| パラメータ | 型 | 必須 | 説明 |
|:---|:---|:---|:---|
| `closing_speech` | `string` | ✅ | お別れの挨拶文言 |

---

### 1.6 非同期ガードレール監視 (FR-07)

| 項目 | 仕様 |
|:---|:---|
| **方式** | LLMベースモデレーション（Chat Completions APIを使用） |
| **モデル** | `llama-3.1-8b-instant`（Groq） |
| **チェック対象** | ユーザー入力テキスト、AI生成テキスト（完了時） |
| **実行** | 非同期（`go m.checkGuardrail(text)`） |
| **タイムアウト** | 5秒 |
| **検出カテゴリ** | ヘイトスピーチ、性的コンテンツ、暴力・自傷脅迫、PII悪用、違法行為指示 |
| **違反時処理** | `content_violation` イベントをフロントエンドに配信 |
| **フォールバック** | API未設定時（キーなし）またはエラー時はパス（安全側に倒す） |

---

### 1.7 会話終了・セッション管理

#### 1.7.1 終了トリガー

| トリガー | 説明 |
|:---|:---|
| **FC `end_conversation`** | AIが会話終了を判断し、`closing_speech` とともにFC発火 |
| **2段階無音タイムアウト** | 60秒無音→確認メッセージ→さらに30秒無音→自動終了 |
| **UI手動操作** | ユーザーが「切断」ボタンをクリック |

#### 1.7.2 2段階無音タイマー

```
[LISTENING] ──60秒無音──> [STAGE 1: 確認メッセージ送信]
                           「まだいらっしゃいますか？」
                           ──30秒無音──> [STAGE 2: セッション終了]
```

- ユーザーの発話やテキスト入力でタイマーリセット
- AI発話中はタイマー一時停止、発話完了後に再開
- 一時停止中に経過した時間は記録され、再開時に残り時間から再開

#### 1.7.3 終了時の処理

1. `silenceTimer.Stop()` でタイマー停止
2. 終了理由に応じた挨拶メッセージをTTSで音声化・送信
3. `session_close` イベント送信
4. 会話履歴を `data/history/` にJSON保存
5. LLMクライアント・WebSocket接続をクローズ

---

### 1.8 セッション状態遷移

フロントエンド（`useSession.ts`）が管理するセッション状態。UIの表示切替とマイク制御に使用する。

| 状態 | 意味 | マイク（pausedフラグ） |
|:---|:---|:---|
| `idle` | WebSocket 未接続 | — |
| `listening` | マイク待受中。ユーザーの発話を検知待ち | `false`（有効） |
| `processing` | STT → LLM 処理中。バックエンドの応答を待っている | `true`（停止） |
| `ai_speaking` | AI発話テキストのストリーミング＋音声再生中 | `false`（有効。割り込み検知のため） |
| `barge_in` | ユーザーが割り込みを行った直後の遷移状態 | 有効（直後に `processing` へ遷移） |
| `completed` | セッション終了。音声再生完了後にリソース解放 | — |
| `error` | 回復不能エラー | — |

#### マイク状態の一元管理

`state` の `watch` により、状態遷移に連動してマイクの pause/resume を一元管理する。

```typescript
watch(state, (newState) => {
  if (newState === 'ai_speaking' || newState === 'listening') {
    voiceInput.resumeListening()  // paused = false
  }
})
```

- `processing` → `ai_speaking` 遷移時に自動でマイク再開（割り込み検知を可能にする）
- `ai_speaking` → `listening` 遷移時にも自動でマイク再開
- `pauseListening()` は `handleSpeechEnd()` 内でのみ呼ばれる（発話送信時）

#### 状態遷移図

```
                    connect
  [idle] ──────────────────────► [listening]
                                   │    ▲
                        発話検知    │    │  音声再生完了 /
                      （pause）    │    │  user_transcript(skipped)
                                   ▼    │
                              [processing]
                                   │
                     text_delta /   │
                     audio_stream   │
                     （resume）     │
                                   ▼
                             [ai_speaking] ◄─── product_recommendation
                                   │
                     音声/テキスト   │
                       割り込み     │
                                   ▼
                              [barge_in] ──► [processing]
                                              （発話終了後）

  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─
  任意の状態 ─── session_close ──► [completed]
  任意の状態 ─── 非回復エラー ───► [error]
```

#### 正常フロー

```
1. [idle] → [listening]
   connect() → WebSocket接続 → connection_state(ready) 受信

2. [listening] → [processing]
   ユーザー発話を検知 → 無音判定 → Opusエンコード
   → handleSpeechEnd() で pauseListening() → 音声データ送信

3. [processing] → [ai_speaking]
   text_delta or audio_stream 受信 → state watch により resumeListening()
   → マイク再開（割り込み検知のため）

4. [ai_speaking] → [listening]
   text_done(is_final=true) 受信 → pendingListeningTransition = true
   → 音声再生完了（isPlaying が false に）→ state = listening
   → state watch により resumeListening()
```

#### バージイン（割り込み）

AI発話中にユーザーが割り込む機能。音声割り込みとテキスト割り込みの2種類がある。

**音声割り込み:**

```
1. [ai_speaking] 中、マイクは有効（paused = false）
2. ユーザーの音声を検知 → onSpeechStart() コールバック
3. handleSpeechStart():
   - audioPlayback.stopAndClear()  // 再生キュー全クリア＋即座に停止
   - state = 'barge_in'
4. バックエンド側:
   - 新しい音声受信 → interruptIfBusy()
   - LLM/TTS パイプラインを cancel
   - clear_audio_buffer メッセージを送信
5. ユーザー発話終了 → Opusエンコード → handleSpeechEnd()
   - pauseListening()
   - state = 'processing'
   - 音声データ送信
```

**テキスト割り込み:**

```
1. sendText() 内で ai_speaking / processing / 音声再生中を検知
2. audioPlayback.stopAndClear()
3. state = 'barge_in'
4. テキスト送信 → state = 'processing'
```

**バックエンドの割り込み処理（interruptIfBusy）:**

ファイル: `backend/internal/session/manager.go`

```go
func (m *Manager) interruptIfBusy() {
    currentState := m.state.Current()
    if currentState != StateAISpeaking && currentState != StateProcessing {
        return
    }
    if m.cancelCurrent != nil {
        m.cancelCurrent()
    }
    m.send(TypeClearAudioBuffer, ClearAudioBuffer{Reason: "barge_in"})
    m.state.Transition(StateListening)
}
```

- `handleAudioSpeech()` と `handleAudioChunk()` の冒頭で呼ばれる
- `Processing` または `AISpeaking` 状態の場合のみ発動
- `context.Cancel()` により進行中のLLMストリーミングとTTS合成を即座に中断

---

## 2. 画面設計書

### 2.1 音声対話コンシェルジュ画面 (`/assistant`)

#### 2.1.1 レイアウト構成

```
┌─────────────────────────────────────────────┐
│ [ヘッダー]                                    │
│  おためしちゃん [接続バッジ] [音量] [履歴] [設定] [接続/切断] │
├─────────────────────────────────────────────┤
│ [ステータスバー]                               │
│  状態表示  セッションID  [復元セッション] [新規会話]│
├─────────────────────────────────────────────┤
│                                             │
│ [対話タイムライン]                              │
│  - AI発話（左寄せ）                            │
│  - ユーザー発話（右寄せ）                       │
│  - 商品カード                                 │
│  - レイテンシ情報                              │
│                                             │
├─────────────────────────────────────────────┤
│ [入力エリア]                                   │
│  [テキスト/マイク切替] [テキスト入力欄] [送信]    │
│  または                                      │
│  [テキスト/マイク切替] [マイクボタン] [状態テキスト]│
└─────────────────────────────────────────────┘
```

#### 2.1.2 入力モード

| モード | UI要素 | 説明 |
|:---|:---|:---|
| **テキスト** | テキスト入力欄 + 送信ボタン | Enterキーで送信。入力欄の左にモード切替トグル |
| **マイク** | マイクボタン（丸型） | タップで音声認識開始/停止。ボタン左にモード切替トグル。音量メーター表示 |

モード切替は入力エリアのトグルスイッチで行う。

#### 2.1.3 セッション復元

会話履歴から復元したセッションでは:
- ステータスバーに「復元セッション」ラベルを表示
- 「新規会話」ボタンを表示（クリックで新しいセッションを開始）
- 復元時は過去の会話ログがタイムラインに表示され、AIが「前回の会話の続きですね」と発話

#### 2.1.4 商品カード

| レイアウト | 条件 |
|:---|:---|
| 中央1枚 | 単一商品紹介 |
| 横並び | まとめ紹介（PC: 3枚 / タブレット: 2枚 / スマホ: スワイプ） |

各カードに画像・商品名・価格・説明を表示。

### 2.2 設定画面 (`/settings`)

| セクション | 設定項目 |
|:---|:---|
| **システムプロンプト** | AIの役割・話し方・会話方針 |
| **LLM** | モデル選択（API `/models` から動的取得） |
| **TTS** | エンジン（Piper/VOICEVOX）、ボイス、話者ID、言語、話速、ノイズパラメータ |
| **STT** | 認識言語、Initial Prompt（固有名詞ヒント） |
| **取扱商品** | 商品の追加・編集・削除（ID、名前、価格、説明、画像URL） |

### 2.3 会話履歴画面 (`/history`)

- 左ペイン: セッション一覧（日時、ターン数、プレビュー）
- 右ペイン: 選択セッションの全会話ログ
- 「この会話から再開」ボタンでアシスタント画面に遷移（`?restore={session_id}`）

---

## 3. API設計書

### 3.1 WebSocket API

**エンドポイント**: `ws://{host}:{port}/ws`

**クエリパラメータ**:

| パラメータ | 説明 |
|:---|:---|
| `restore_session_id` | 復元するセッションのUUID |
| `recommendation_mode` | レコメンドモード |
| `post_recommendation_behavior` | レコメンド後挙動 |

**Ping/Pong キープアライブ**:
- サーバーから30秒間隔でPingを送信
- クライアントのPong応答で読み取りデッドラインを90秒に更新

#### 3.1.1 クライアント→サーバー（Inbound）

| Type | ペイロード | 説明 |
|:---|:---|:---|
| `audio_chunk` | `{text?, audio_data?, duration_ms?}` | テキスト入力またはオーディオチャンク |
| `audio_speech` | `{audio}` | Base64 PCM音声データ（16kHz, 16bit, mono）。サーバー側でSTT処理 |
| `session_control` | `{action, reason?, config?}` | `action: "close"` でセッション終了要求 |

#### 3.1.2 サーバー→クライアント（Outbound）

| Type | ペイロード | 説明 |
|:---|:---|:---|
| `connection_state` | `{state, session_id}` | 接続確立通知 |
| `text_delta` | `{text}` | LLMテキストストリーム断片（文単位） |
| `text_done` | `{text, audio_chunk?, is_final, latency?}` | テキスト完了通知。`latency` に計測情報 |
| `audio_stream` | `{audio, sentence_id, is_final?}` | TTS音声チャンク（Base64 Opus） |
| `product_recommendation` | `{transcript, audio_chunk?, products[]}` | 商品レコメンド |
| `clear_audio_buffer` | `{reason}` | 音声バッファクリア指示（barge-in時） |
| `user_transcript` | `{text, skipped?}` | STT結果通知 |
| `content_violation` | `{reason, message}` | ガードレール違反通知 |
| `error` | `{code, message, recoverable}` | エラー通知 |
| `session_close` | `{reason, message}` | セッション終了通知 |
| `history_restore` | `{messages[{role, content}]}` | 復元履歴データ |

**LatencyInfo構造体**:

```json
{
  "stt_ms": 0,
  "llm_ttfb_ms": 287,
  "llm_total_ms": 347,
  "tts_ms": 210,
  "total_ms": 509
}
```

### 3.2 REST API

| メソッド | パス | 説明 |
|:---|:---|:---|
| `GET` | `/api/config` | 現在の設定を取得 |
| `PUT` | `/api/config` | 設定を更新（1MB制限） |
| `GET` | `/api/models` | 利用可能なLLMモデル一覧を取得 |
| `GET` | `/api/tts/voices` | 利用可能なTTSボイス一覧を取得 |
| `GET` | `/api/history` | セッション履歴一覧を取得 |
| `GET` | `/api/history/{id}` | 特定セッションの詳細を取得（UUID形式のバリデーション付き） |
| `GET` | `/health` | ヘルスチェック |

**CORS設定**: `CORS_ORIGIN` 環境変数（デフォルト: `http://localhost:3000`）。WebSocket `CheckOrigin` も同一オリジンを検証。

### 3.3 外部TTS API

#### Piper Plus

```
POST {TTS_URL}/synthesize
Content-Type: application/json

{
  "text": "こんにちは",
  "voice": "base_6lang",
  "speaker_id": 0,
  "language": "ja",
  "length_scale": 1.25,
  "noise_scale": 0.667,
  "noise_w": 0.8
}
```

レスポンス: WAV音声 → Opus変換後にクライアントへ配信

**言語別デフォルト話者ID**: ja=0, en=20, zh=330, es=472, fr=535, pt=563

#### VOICEVOX

```
# Step 1: 音声クエリ生成
POST {VOICEVOX_URL}/audio_query?text={text}&speaker={speaker_id}

# Step 2: 音声合成
POST {VOICEVOX_URL}/synthesis?speaker={speaker_id}
Content-Type: application/json
Body: {Step 1のレスポンス（speedScale適用後）}
```

### 3.4 STT API

設定画面でエンジンを切り替え可能。

| エンジン | 設定値 | 説明 |
|:---|:---|:---|
| faster-whisper | `whisper` | ローカルのfaster-whisperサーバー（CTranslate2ベース）。Opus/WebM対応 |
| Groq | `groq` | Groq APIを使用。モデル選択可能（whisper-large-v3-turbo等） |

#### faster-whisper

```
POST {STT_URL}/inference
Content-Type: multipart/form-data

file: audio.webm (Opus/WebM)
response_format: verbose_json
temperature: 0.0
language: {config値 or 省略(auto)}
prompt: {config stt_prompt}
```

レスポンス: `{text, language}`

#### Groq STT

```
POST https://api.groq.com/openai/v1/audio/transcriptions
Content-Type: multipart/form-data

file: audio.webm (Opus/WebM)
model: {config stt_model} (デフォルト: whisper-large-v3-turbo)
language: {config値 or 省略(auto)}
prompt: {config stt_prompt}
```

選択可能モデル: `whisper-large-v3-turbo`, `whisper-large-v3`, `distil-whisper-large-v3-en`

#### 音声フォーマット

- フロントエンド → バックエンド: Opus/WebM（Base64）
- バックエンド → STT: Opus/WebMバイナリをそのまま送信（WAV変換不要）
- faster-whisper: 内部でffmpegにより自動変換
- Groq API: Opus/WebMネイティブ対応

---

## 4. 音声入力パイプライン

### 4.1 フロントエンド音声キャプチャ

```
マイク（48kHz）
  → AudioWorklet（128サンプル/フレーム ≈ 2.67ms）
  → RNNoise（ノイズ抑制）
  → RMS音量判定
  → 発話区間検出
  → Opus/WebMエンコード（非同期）
  → Base64 → onSpeechEnd コールバック
```

#### VAD（音声区間検出）パラメータ

| パラメータ | 値 | 説明 |
|:---|:---|:---|
| `SILENCE_THRESHOLD` | 0.015 | RMS閾値。これを超えると発話と判定 |
| `SILENCE_FRAMES_REQUIRED` | 30 | 発話終了と判定する連続無音フレーム数（約80ms） |
| `MIN_SPEECH_FRAMES` | 5 | 最低発話フレーム数（約13ms）。これ未満は破棄 |

- 音量ベースの簡易VAD。文単位や意味的な区切りではない
- `echoCancellation: true` と RNNoise の組み合わせでAI再生音のフィードバックを抑制

#### 音声エンコード

- RNNoise処理済みのPCMデータを `OfflineAudioContext` → `MediaRecorder`（`audio/webm;codecs=opus`、32kbps）でエンコード
- エンコードは非同期（fire-and-forget）。エンコード完了後に `onSpeechEnd` コールバックが呼ばれる

### 4.2 音声再生パイプライン

```
audio_stream / text_done.audio_chunk（Base64）
  → decodeAudioData → AudioBuffer キュー（最大50）
  → 順次再生（source.onended → playNext）
```

| 機能 | メソッド | 説明 |
|:---|:---|:---|
| 再生 | `playAudio(base64)` | デコードしてキューに追加、未再生なら再生開始 |
| 割り込み停止 | `stopAndClear()` | キュー全クリア＋現在の再生を即停止 |
| 完了待ち | `waitUntilDone()` | キュー＋再生＋デコード待ちがすべて完了するまで待機 |
| 音量制御 | `setVolume(val)` / `toggleMute()` | `GainNode` で音量調整 |

### 4.3 サーバー側STT処理

1. Base64 Opus/WebMデータをデコード
2. Opus/WebMバイナリをそのままSTTエンジンへ送信（WAV変換不要）
3. ノイズテキストフィルタリング（誤認識パターン除外、3文字以下除外）
4. 検出言語を記録（TTS言語自動追従用）

### 4.4 TTS区切り単位

LLMのストリーミング出力を句読点で区切り、文単位でTTSに投入する。

| ルール | 条件 | 動作 |
|:---|:---|:---|
| 句読点区切り | `。` `、` `！` `？` `…` を検出 | その位置で分割してTTSに送信 |
| 文字数上限 | 100文字を超過 | 句読点がなくても強制分割 |
| タイムアウト | 1秒間新しいトークンが来ない | バッファ内の残りを強制送信 |
| LLM完了 | `text_done` イベント | `Flush()` で残りを送信 |

句読点で細かく区切ることで、音声再生開始までのレイテンシを最小化する。

---

## 5. 設定管理

### 5.1 設定ファイル (`data/config.yaml`)

```yaml
system_prompt: |-
  [Personal]
  私は「おためしちゃん」というアシスタントです。
  ...
llm_model: llama-3.1-8b-instant
tts_engine: piper
tts_voice: base_6lang
tts_speaker_id: 0
tts_language: ja
tts_length_scale: 1.25
tts_noise_scale: 0.667
tts_noise_w: 0.8
voicevox_speaker_id: 1
voicevox_speed_scale: 1.2
stt_language: ja
stt_prompt: ""
products:
  - id: h001
    name: ハイチュウ グレープ
    price: 140
    description: ...
```

### 5.2 環境変数

| 変数 | 説明 | デフォルト |
|:---|:---|:---|
| `LLM_MODE` | LLMモード（`stub` / `chat`） | `stub` |
| `OPENAI_API_KEY` | LLM APIキー | — |
| `OPENAI_BASE_URL` | LLM APIエンドポイント | `https://api.openai.com/v1` |
| `OPENAI_MODEL` | LLMモデル（config優先） | `gpt-4o-mini` |
| `TTS_URL` | Piper TTS URL | — |
| `TTS_ENGINE` | TTSエンジン（config優先） | `piper` |
| `VOICEVOX_URL` | VOICEVOX URL | `http://voicevox:50021` |
| `STT_URL` | Whisper STT URL | — |
| `STT_LANGUAGE` | STT言語（config優先） | `ja` |
| `CONFIG_PATH` | 設定ファイルパス | `data/config.yaml` |
| `CORS_ORIGIN` | CORS許可オリジン | `http://localhost:3000` |
| `PORT` | バックエンドポート | `8080` |
| `LOG_PATH` | ログファイルパス | — |
| `LOG_LEVEL` | ログレベル | `info` |

---

## 6. サービス構成

### 6.1 Docker Compose サービス一覧

| サービス | ポート | 役割 | 依存 |
|:---|:---|:---|:---|
| `frontend` | 3000 | Nuxt 3 SPA（音声入力UI、会話表示） | backend |
| `backend` | 8080 | Go WebSocket APIサーバー（マルチステージビルド） | tts, voicevox, stt |
| `tts` | 5000 | Piper Plus TTS（多言語対応） | — |
| `voicevox` | 50021 | VOICEVOX TTS（日本語高品質音声） | — |
| `stt` | 8888 | Whisper.cpp STT | — |

### 6.2 ヘルスチェック

| サービス | チェック方法 | 間隔 | 起動猶予 |
|:---|:---|:---|:---|
| `tts` | HTTP GET `/health` | 10s | — |
| `voicevox` | wget `/version` | 10s | 120s |
| `stt` | curl `/health` | 10s | 60s |

---

## 7. ディレクトリ構成

```
├── backend/                  # Go バックエンド
│   ├── cmd/server/           # エントリポイント (main.go)
│   ├── internal/
│   │   ├── catalog/          # 商品カタログ管理
│   │   ├── config/           # 設定管理 (YAML永続化, mutex保護)
│   │   ├── guardrail/        # コンテンツモデレーション (LLMベース)
│   │   ├── handler/          # HTTP/WS ハンドラ
│   │   ├── history/          # 会話履歴保存 (JSON)
│   │   ├── logging/          # ログ設定
│   │   ├── openai/           # LLM クライアント (Chat Completions SSE)
│   │   ├── protocol/         # WebSocket メッセージ定義
│   │   ├── session/          # セッション管理・状態遷移・無音タイマー
│   │   ├── stt/              # 音声認識 (Whisper.cpp)
│   │   ├── textbuf/          # テキストバッファ（文単位分割）
│   │   └── tts/              # 音声合成 (Piper/VOICEVOX/Stub)
│   ├── data/                 # 設定・履歴データ
│   ├── Dockerfile            # マルチステージビルド (golang:1.23-alpine → alpine:3.20)
│   └── entrypoint.sh
├── frontend/                 # Nuxt 3 フロントエンド (SPA, ssr: false)
│   ├── pages/                # assistant, settings, history, index
│   ├── composables/          # useSession, useWebSocket, useVoiceInput,
│   │                         # useAudioPlayback, useAudioCapture,
│   │                         # useNoiseSuppression, useAppSettings
│   ├── components/           # ConnectionBadge, StatusIndicator,
│   │                         # ConversationTimeline, ProductCard,
│   │                         # ProductCardList, VolumeMeter, ErrorBanner,
│   │                         # SessionConfigPanel
│   └── types/                # TypeScript 型定義
├── tts/                      # Piper Plus TTS サーバー (Python)
├── stt/                      # Whisper.cpp STT サーバー
├── docs/                     # ドキュメント
└── docker-compose.yml
```

---

## 8. セキュリティ

| 対策 | 実装 |
|:---|:---|
| **CORS制御** | `CORS_ORIGIN` 環境変数によるオリジン制限 |
| **WebSocket CheckOrigin** | CORS_ORIGINと同一のオリジン検証 |
| **リクエストボディ制限** | 設定API: 1MB (`io.LimitReader`) |
| **入力バリデーション** | 履歴APIのセッションID: UUID正規表現チェック |
| **APIキー保護** | `.env` ファイルに格納、`.gitignore` で除外 |
| **コンテンツモデレーション** | LLMベースの非同期ガードレール監視 |
| **同時書き込み保護** | gorilla/websocket の書き込みは単一ゴルーチン（sendCh経由） |
| **設定ロック** | config の Load/Get/Update は mutex で保護 |

---

## 9. 会話フローチャート

```mermaid
flowchart TD
    A[セッション開始] --> B{復元セッション?}
    B -- Yes --> C[履歴復元 + 再開メッセージ]
    B -- No --> D[挨拶生成]
    C --> E[LISTENING]
    D --> E

    E --> F{ユーザー入力}
    F -- テキスト --> G[ガードレールチェック（非同期）]
    F -- 音声 --> H[STT変換]
    H --> I{ノイズ判定}
    I -- ノイズ --> E
    I -- 有効テキスト --> G

    G --> J[LLM処理 PROCESSING]
    J --> K{LLM応答}
    K -- text_delta --> L[テキストバッファリング → TTS → 音声配信]
    K -- recommend_product --> M[商品カード + TTS音声配信]
    K -- end_conversation --> N[終了処理]
    K -- error --> O[エラー通知]

    L --> P[AI_SPEAKING]
    P --> Q{Barge-in?}
    Q -- Yes --> R[キャンセル + clear_audio_buffer]
    R --> E
    Q -- No --> S[text_done]
    S --> E

    M --> T{post_recommendation?}
    T -- ask_interest --> U[フォローアップ発話]
    T -- return_to_conversation --> E
    U --> E

    E --> V{無音60秒}
    V --> W[確認メッセージ]
    W --> X{無音30秒}
    X --> N

    N --> Y[終了挨拶TTS + session_close]
    Y --> Z[履歴保存 → CLOSED]
```

---

**ドキュメントバージョン**: v6.0
**最終更新**: 2026-08-05
**前バージョンからの主な変更点**:
- OpenAI Realtime API → Chat Completions API（SSEストリーミング）に変更
- TTS: 未決定 → Piper Plus / VOICEVOX 実装済
- STT: Whisper.cpp 実装済
- ガードレール: OpenAI Moderation API → LLMベースモデレーション
- 無音タイマー: 60秒 + 30秒
- 音声キャプチャ: AudioWorklet API + RNNoise（WASM）
- セッション復元時の「新規会話」ボタン追加
- 入力モード切替をステータスバーから入力エリアに移動
- マルチステージDockerビルド導入
- WebSocket Ping/Pongキープアライブ追加
