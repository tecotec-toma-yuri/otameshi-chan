# 商品おすすめ機能 詳細仕様書

## 1. 機能概要

ユーザーとの音声/テキスト会話の中で、AIが適切なタイミングで商品をおすすめし、商品カードとして視覚的に表示する機能。LLMのFunction Callingを使い、会話文脈に基づいて商品を選定・紹介する。

---

## 2. あるべき仕様

本章では、この機能が理想的に動作した場合の仕様を定義する。

### 2.1 ユーザー体験

#### 基本シナリオ

```
1. ユーザーがAIと会話を始める
2. AIが自然な流れでユーザーの好みを探る質問をする
   （「どんな味が好きですか？」「今日はどんな気分ですか？」等）
3. ユーザーが回答する。会話の中で十分に好みを把握する
4. AIが把握した好みに基づいて商品を紹介する
   （「甘いフルーツ味がお好きなんですね！こちらはいかがですか？」）
5. 画面に商品カード（画像・名前・説明）が表示される
6. ユーザーが商品について質問したり、別のおすすめを求めたりできる
7. 会話が終わったらAIが自然にお別れのセリフを言って終了する
```

**ポイント:** ステップ2〜3はAIが主導する。いきなり商品を紹介するのではなく、会話の中で自然に好みを引き出してから推薦に移行する。

#### ユーザーが期待する動作

| 場面 | AIの動作 |
|---|---|
| 会話の序盤 | 好みを探る質問を自然に投げかける（「どんな味がお好きですか？」「甘いもの派ですか？」） |
| 「おすすめは？」「何がある？」 | すぐに商品を推薦する |
| 会話を通じて好みを十分に把握した後 | 把握した好みに基づき、自然な流れで推薦に移行する |
| 特定の味や種類に言及した時 | 関連商品を推薦する |
| 挨拶のみ・まだ好みが不明の段階 | 推薦しない。好みを探る質問で会話を続ける |
| 推薦後「もういい」「大丈夫」 | 推薦を止め、会話に戻るか終了する |
| 「さようなら」「ありがとう」 | 会話の文脈に合ったお別れのセリフで終了する |

### 2.2 商品データ

#### 商品マスタ

```yaml
products:
  - id: "h001"
    name: "ハイチュウ ＜グレープ＞"
    description: "ジューシーな果汁感あふれる定番フレーバー"
    image_url: "/images/h001.png"
    tags: ["甘い", "フルーティー", "定番"]
```

| フィールド | 型 | 必須 | 説明 |
|---|---|---|---|
| `id` | `string` | ○ | 一意の商品ID |
| `name` | `string` | ○ | 商品名 |
| `description` | `string` | ○ | 商品の説明文 |
| `image_url` | `string` | — | 商品画像のパス |
| `tags` | `string[]` | — | 味や特徴のタグ。LLMが好みとのマッチングに使用 |
| `related_product_ids` | `string[]` | — | シーケンシャルモード時に続けて紹介する関連商品のIDリスト（順序あり）。設定画面で管理する |

- 商品が1件以上登録されている場合に推薦機能が有効になる
- 商品が0件の場合、推薦機能は完全にOFFとなる:
  - `assess_interest` はLLMに登録されない（第1段階の興味判定自体が行われない）
  - `recommend_product` も当然登録されない
  - システムプロンプトに興味判定ガイドラインは追記されない
  - 通常の会話処理のみが行われる

#### 興味判定の設定

| 設定項目 | 型 | デフォルト値 | 説明 |
|---|---|---|---|
| `interest_prompt` | `string` | （後述） | 興味判定のためのプロンプト。どのような話題・キーワードを興味ありと見なすかを自由記述で指定する |
| `interest_threshold` | `int` | 3 | 第2段階に移行する興味度の閾値（1〜5） |

`interest_prompt` は設定画面のテキストエリアで入力でき、第1段階のシステムプロンプトにそのまま注入される。運用者が商品ジャンルや対象顧客に合わせて判定基準をカスタマイズできる。

**`interest_prompt` の記述例:**

```
あなたはお菓子の販売員です。
以下のような話題が出た場合に、お客様が商品に興味を持っていると判定してください。

- お菓子、スイーツ、甘いもの、おやつに関する話題
- 味の好み（甘い、酸っぱい、フルーティー等）への言及
- 「何かおすすめある？」「何を売ってるの？」等の直接的な質問
- 食べたい、試したい、買いたいといった意欲の表現
- 贈り物やお土産を探しているという話題
```

**デフォルトの `interest_prompt`:**

```
以下のような話題が出た場合に、お客様が商品に興味を持っていると判定してください。
- 商品に関連する話題への言及
- 味や好みへの言及
- 「おすすめは？」「何がある？」等の直接的な質問
- 購入意欲の表現
```

### 2.3 推薦の判断とトリガー

推薦には2つのモードがあり、設定画面で切り替える。

| モード | 概要 |
|---|---|
| `ai_driven` | 会話の中でAIが興味を判定し、閾値を超えたら推薦する |
| `sequential` | 事前に設定した商品リストを順番に紹介する |

#### 推薦モード: AI駆動（`ai_driven`）

推薦は**2段階のプロセス**で行う。段階ごとに別のLLMリクエストとなる。

##### 第1段階: 興味判定（`assess_interest`）

通常の会話応答と並行して、AIがユーザーの発言から商品への興味度を5段階で判定する。

**この関数は通常の会話応答と同時に呼ばれる。** AIは会話を続けながら（応答テキストを返しながら）、同時に `assess_interest` を呼び出す。つまりユーザーから見ると自然な会話が続いているが、裏側では興味判定が走っている。

**興味度の5段階:**

| レベル | 意味 | 例 |
|---|---|---|
| 1 | 興味なし | 挨拶のみ、商品と無関係な雑談 |
| 2 | わずかに関連 | 「お菓子」という単語が出た程度 |
| 3 | やや興味あり | 好みや気分に言及（「甘いもの好き」「最近チョコにハマってる」） |
| 4 | 明確な興味 | 「おすすめは？」「何がありますか？」 |
| 5 | 強い購買意欲 | 「買いたい」「どれがいい？」「試してみたい」 |

**バックエンド側の閾値制御:**
- バックエンドに `interest_threshold`（デフォルト: 3）を設定する
- `interest_level >= interest_threshold` の場合 → 第2段階に移行
- `interest_level < interest_threshold` の場合 → 何もしない（会話を継続）
- 閾値は設定画面または環境変数で変更可能とする

**第2段階への移行時の処理:**
- 興味判定結果（レベル・検出した好み）をログに記録
- **システムプロンプトに商品一覧を注入**する（この時点で初めて商品情報がLLMに渡る）
- `assess_interest` の返却テキスト（「○○に興味があるんですね！」等）をTTS変換してユーザーに返す
- 続けて `recommend_product` を呼ぶLLMリクエストを発行する

##### 第2段階: 商品選定・推薦（`recommend_product`）

第1段階で閾値を超えた場合にのみ実行される。
システムプロンプトに商品一覧が注入された状態で、LLMが好みに最も合う1商品を**必ず選定**する。

**推薦時の制約:**
- 1回の推薦では必ず1商品とする
- 推薦理由を添える（「甘いのがお好きとのことでしたので」等）
- 紹介セリフは2文以内で簡潔に

**重要な設計方針:**
- 第1段階の時点では商品一覧はシステムプロンプトに含まれない → LLMが商品を知らない状態で興味判定を行うため、不必要な推薦が発生しない
- 第2段階では商品一覧が注入済みなので、LLMは必ず最適な商品を選べる
- 第2段階の関数は「必ず1商品を選ぶ」設計であり、「推薦しない」という選択肢はない

### 2.4 LLM Function Calling

#### 第1段階で登録するツール

セッション開始時から登録される。商品一覧はこの時点ではシステムプロンプトに含まれない。

##### assess_interest

通常の会話応答と並行して呼び出される。ユーザーの商品への興味度を5段階で判定する。

```json
{
  "name": "assess_interest",
  "description": "会話の応答と同時に、お客様の商品への興味度を判定する。毎回の会話で呼び出し、興味の度合いを1〜5で返す。応答テキストとは別に、この関数も同時に呼ぶこと。",
  "parameters": {
    "type": "object",
    "properties": {
      "interest_level": {
        "type": "integer",
        "minimum": 1,
        "maximum": 5,
        "description": "興味の度合い（1=興味なし 2=わずかに関連 3=やや興味あり 4=明確な興味 5=強い購買意欲）"
      },
      "detected_preferences": {
        "type": "array",
        "items": {"type": "string"},
        "description": "会話から検出したユーザーの好み（例: [\"甘い\", \"フルーティー\"]）。興味が低い場合は空配列"
      },
      "response_text": {
        "type": "string",
        "description": "興味度が高い場合にユーザーに返す反応（例: 「甘いものがお好きなんですね！」）。興味が低い場合は空文字"
      },
      "trigger_utterance": {
        "type": "string",
        "description": "判定の根拠となったユーザーの発言の要約"
      }
    },
    "required": ["interest_level", "detected_preferences", "response_text", "trigger_utterance"]
  }
}
```

#### 第2段階で登録するツール

`assess_interest` の結果が閾値を超えた後、商品一覧とともにシステムプロンプトに注入され、新たに登録される。

##### recommend_product

商品一覧の中から、ユーザーの好みに最も合う1商品を必ず選定する。

```json
{
  "name": "recommend_product",
  "description": "商品一覧の中から、お客様の好みに最も合う商品を1つ選んで紹介する。必ず1商品を選ぶこと。",
  "parameters": {
    "type": "object",
    "properties": {
      "product_id": {
        "type": "string",
        "enum": ["h001", "h002", "h003"],
        "description": "おすすめする商品のID（※ enum は商品マスタの id 一覧から都度動的に生成する）"
      },
      "introduction_speech": {
        "type": "string",
        "description": "商品を紹介するときの発話テキスト（2文以内）"
      },
      "reason": {
        "type": "string",
        "description": "推薦理由（会話から把握した好みとの関連）"
      }
    },
    "required": ["product_id", "introduction_speech", "reason"]
  }
}
```

**注意:** `product_ids`（配列）ではなく `product_id`（単一文字列）とする。必ず1商品であることを型レベルで保証する。

#### end_conversation

ユーザーが会話終了の意思を示した際に呼び出す関数。

```json
{
  "name": "end_conversation",
  "description": "会話を終了する。お客様が会話を終えたい意思を示した時に使う。",
  "parameters": {
    "type": "object",
    "properties": {
      "closing_speech": {
        "type": "string",
        "description": "終了時に読み上げるお別れのセリフ"
      }
    },
    "required": ["closing_speech"]
  }
}
```

- `closing_speech` はLLMが会話の文脈に合わせて生成する
- ハードコードされた定型文ではなく、**LLM生成のセリフを使用する**

### 2.5 システムプロンプト

システムプロンプトは段階に応じて変化する。

#### 第1段階のシステムプロンプト（セッション開始時）

商品一覧は含めない。興味判定のガイドラインのみ追記する。
`{interest_prompt}` の部分には設定画面で入力された `interest_prompt` がそのまま挿入される。

```
## 興味判定ガイドライン

あなたは通常の会話応答と同時に、お客様の商品への興味度を判定します。
毎回の応答で assess_interest 関数を呼び出し、興味度を1〜5で判定してください。

### 判定基準

{interest_prompt}

### 興味度の5段階
- 1: 興味なし（上記の判定基準に該当しない）
- 2: わずかに関連（判定基準にかすかに触れる程度）
- 3: やや興味あり（判定基準に該当する話題が出ている）
- 4: 明確な興味（直接的に商品やおすすめを求めている）
- 5: 強い購買意欲（買いたい・試したい等の意欲を示している）

### response_text の書き方
- 興味度3以上の場合: お客様の興味に反応する一言を返す
  （「甘いものがお好きなんですね！」「お菓子に興味がおありですか？」等）
- 興味度1〜2の場合: 空文字を返す

## 関数の使い方
- assess_interest: 毎回の応答で呼び出す。通常の会話応答テキストとは別に、
  この関数も同時に呼ぶこと。
- end_conversation: お客様が「さようなら」「ありがとう、もう大丈夫」など
  会話を終えたい意思を明確に示した時に使う。
```

#### 第2段階のシステムプロンプト（興味判定の閾値超過後に注入）

商品一覧と推薦ガイドラインを追加注入する。

```
## 取扱商品一覧
- h001: ハイチュウ ＜グレープ＞ [甘い,フルーティー,定番]
  ジューシーな果汁感あふれる定番フレーバー
- h002: ハイチュウ ＜ストロベリー＞ [甘酸っぱい,フルーティー,定番]
  いちごの華やかな香りと甘酸っぱさ
- h003: ...

## 商品推薦ガイドライン
お客様の好みに最も合う商品を1つ選び、recommend_product 関数で紹介してください。
- 必ず1商品を選ぶこと
- なぜその商品を薦めるのか理由を添える（「甘いのがお好きとのことでしたので」等）
- introduction_speech は2文以内で簡潔に
```

### 2.6 バックエンド処理フロー

#### 全体の流れ

```
[セッション開始]
  │
  │  システムプロンプト: 興味判定ガイドラインのみ（商品一覧なし）
  │  登録ツール: assess_interest, end_conversation
  │
  ▼
[通常会話ループ]
  │
  │  ユーザー発話 → LLM応答（テキスト + assess_interest 同時呼出し）
  │       │
  │       ├─ interest_level < 閾値 → ログ記録のみ、会話継続
  │       │
  │       └─ interest_level >= 閾値 → 第2段階へ移行 ──┐
  │                                                    │
  ▼                                                    ▼
[第2段階: 商品推薦]
  │
  │  1. 興味判定結果をログに記録
  │  2. response_text をTTS変換してユーザーに返す
  │  3. システムプロンプトに商品一覧を注入
  │  4. 登録ツールに recommend_product を追加
  │  5. LLMリクエストを発行（recommend_product を必ず呼ばせる）
  │
  ▼
[recommend_product 処理]
  │
  ├─ 1. 引数をパース（product_id, introduction_speech, reason）
  ├─ 2. product_id から商品情報を取得
  ├─ 3. introduction_speech をTTSで音声合成
  ├─ 4. product_recommendation メッセージを送信
  │       └─ 商品情報 + 音声 + 推薦理由
  ├─ 5. 構造化ログに推薦イベントを記録
  ├─ 6. 状態を Listening に遷移、無音タイマーを再開
  └─ 7. 推薦後の動作を実行（設定に応じて）
```

#### 閾値の設定

| 設定項目 | デフォルト値 | 説明 |
|---|---|---|
| `interest_threshold` | 3 | この値以上の `interest_level` で第2段階に移行 |

- 設定画面または環境変数（`INTEREST_THRESHOLD`）で変更可能
- 閾値を上げると推薦が慎重に、下げると積極的になる

#### 推薦モード: シーケンシャル（`sequential`）

興味判定をきっかけに商品を推薦し、その後に**紐づいた関連商品を続けて紹介**するモード。店頭デモや展示会など、関連商品をまとめて案内したい場面で使用する。

**AI駆動モードとの共通点:**
- 第1段階（`assess_interest`）は同一。興味判定なしにいきなり商品を紹介することはない
- 興味判定の閾値制御も共通

**AI駆動モードとの違い:**
- 第2段階で `recommend_product` により商品が選定された後、その商品に紐づいた関連商品を**ユーザーの発話を挟まず連続で紹介**する
- 関連商品の紐づけは設定画面で管理する

**商品の紐づけ（`related_product_ids`）:**

商品マスタに `related_product_ids` フィールドを追加する。設定画面で商品ごとに関連商品を設定できる。

```yaml
products:
  - id: "h001"
    name: "ハイチュウ ＜グレープ＞"
    description: "ジューシーな果汁感あふれる定番フレーバー"
    image_url: "/images/h001.png"
    tags: ["甘い", "フルーティー", "定番"]
    related_product_ids: ["h002", "h003"]
  - id: "h002"
    name: "ハイチュウ ＜ストロベリー＞"
    description: "いちごの華やかな香りと甘酸っぱさ"
    image_url: "/images/h002.png"
    tags: ["甘酸っぱい", "フルーティー", "定番"]
    related_product_ids: []
```

**動作フロー:**

```
[セッション開始]
  │
  │  第1段階と同じ: assess_interest + end_conversation を登録
  │  システムプロンプト: 興味判定ガイドラインのみ（商品一覧なし）
  │
  ▼
[通常会話ループ — AI駆動と同一]
  │
  │  ユーザー発話 → LLM応答 + assess_interest
  │       │
  │       └─ interest_level >= 閾値 → 第2段階へ
  │
  ▼
[第2段階: 商品選定]
  │
  │  商品一覧を注入 → recommend_product で1商品を選定（パターンB と同一）
  │  例: h001 が選定される
  │
  ▼
[h001 を紹介]
  │
  │  introduction_speech をTTS → product_recommendation 送信
  │  推薦後の動作を実行
  │
  ▼
[h001 の related_product_ids を確認]
  │
  │  related_product_ids: ["h002", "h003"]
  │
  ├─ h002 を紹介（LLMリクエスト: パターンE）
  │    └─ 推薦後の動作を実行
  │
  ├─ h003 を紹介（LLMリクエスト: パターンE）
  │    └─ 推薦後の動作を実行
  │
  └─ 全関連商品紹介済み → 通常会話ループに戻る
```

**補足:**
- 関連商品が0件の場合は、AI駆動モードと同じ動作になる（1商品紹介して終了）
- 関連商品の紹介順は `related_product_ids` の配列順に従う
- 関連商品の紹介中もユーザーの発話は受け付ける（割り込み可能とするかは要検討）

#### 推薦後の動作

設定画面で選択可能。両モード（AI駆動・シーケンシャル）共通で適用される。

| 設定値 | 動作 |
|---|---|
| `return_to_conversation` | 何もせず通常会話に戻る。ユーザーの次の発話を待つ |
| `ask_interest` | 「気になるものはありましたか？」を音声付きで送信し、ユーザーの反応を促す |

### 2.7 WebSocket プロトコル

#### セッション設定（Client → Server）

```json
{
  "type": "session_control",
  "action": "start",
  "config": {
    "recommendation_mode": "ai_driven",
    "post_recommendation_behavior": "return_to_conversation"
  }
}
```

- `recommendation_mode`: `"ai_driven"` または `"sequential"`（設定画面で選択）
- `post_recommendation_behavior`: `"return_to_conversation"` または `"ask_interest"`（設定画面で選択、両モード共通）
- シーケンシャルモードの関連商品紐づけは商品マスタの `related_product_ids` で管理（セッション設定ではなく商品データ側）

#### 商品推薦メッセージ（Server → Client）

```json
{
  "type": "product_recommendation",
  "transcript": "甘いのがお好きなんですね！こちらはいかがですか？",
  "audio_chunk": "<Base64 音声>",
  "reason": "甘いフルーツ味がお好きとのことでしたので",
  "products": [
    {
      "product_id": "h001",
      "name": "ハイチュウ ＜グレープ＞",
      "description": "ジューシーな果汁感あふれる定番フレーバー",
      "image_url": "/images/h001.png",
      "tags": ["甘い", "フルーティー", "定番"]
    }
  ]
}
```

#### セッション終了メッセージ（Server → Client）

```json
{
  "type": "session_close",
  "reason": "conversation_ended",
  "message": "ありがとうございました！またお気軽にお声がけくださいね。"
}
```

- `message` にはLLMが生成した `closing_speech` を使用する

### 2.8 フロントエンド表示

#### 商品カード（ProductCard）

```
┌──────────────────────┐
│  ┌──────────────────┐ │
│  │   [商品画像]      │ │  ← image_url がある場合は実画像
│  │                  │ │     ない場合はグラデーション背景
│  └──────────────────┘ │
│                      │
│  ハイチュウ ＜グレープ＞│
│  ¥140                │
│  ジューシーな果汁感... │
│                      │
│      [詳細を見る]     │  ← モーダルで詳細表示
└──────────────────────┘
```

**表示要素:**
- 商品画像（なければグラデーション背景にフォールバック）
- 商品名
- 説明文（2行まで、超過時は省略）
- 「詳細を見る」ボタン → モーダルで全情報を表示

#### 商品カードリスト（ProductCardList）

- 推薦理由をカード群の上部に表示（「甘いフルーツ味がお好きとのことでしたので」）
- 1商品: 中央寄せ
- 複数商品: レスポンシブグリッド（1列 → sm:2列 → lg:3列）

#### 商品詳細モーダル（ProductDetailModal）

```
┌──────────────────────────────┐
│  ハイチュウ ＜グレープ＞       │
│  ┌────────────────────────┐  │
│  │       [商品画像]        │  │
│  └────────────────────────┘  │
│                              │
│  タグ: 甘い / フルーティー     │
│                              │
│  ジューシーな果汁感あふれる    │
│  定番フレーバー               │
│                              │
│  推薦理由:                    │
│  「甘いフルーツ味がお好きと    │
│    のことでしたので」          │
│                              │
│          [閉じる]             │
└──────────────────────────────┘
```

### 2.9 推薦ログ

バックエンドで推薦イベントを構造化ログに記録する。

```json
{
  "level": "INFO",
  "msg": "product_recommended",
  "session_id": "sess_abc123",
  "product_ids": ["h001", "h003"],
  "reason": "甘いフルーツ味がお好きとのことでしたので",
  "introduction_speech": "甘いのがお好きなんですね！こちらはいかがですか？"
}
```

### 2.10 LLMリクエストパターン一覧

本節では、推薦機能に関わるすべてのLLMリクエストパターンを、具体的なリクエスト/レスポンス例とともに示す。

---

#### パターンA: 通常会話 + 興味判定（第1段階）

**発生条件:** 商品が1件以上登録されている状態での、毎回のユーザー発話

ユーザーの発話に対する通常の会話応答と、`assess_interest` の並行呼び出しが同一リクエスト内で行われる。

**リクエスト:**

```json
{
  "model": "llama-3.3-70b-versatile",
  "stream": true,
  "messages": [
    {
      "role": "system",
      "content": "[Personal]\n私は「おためしちゃん」というアシスタントです。...\n\n## 興味判定ガイドライン\nあなたは通常の会話応答と同時に、お客様の商品への興味度を判定します。\n毎回の応答で assess_interest 関数を呼び出し、興味度を1〜5で判定してください。\n\n### 判定基準\n{interest_prompt の内容がここに展開される}\n\n### 興味度の5段階\n- 1: 興味なし...\n...\n\n## 関数の使い方\n- assess_interest: 毎回の応答で呼び出す。通常の会話応答テキストとは別に、この関数も同時に呼ぶこと。\n- end_conversation: お客様が会話を終えたい意思を示した時に使う。"
    },
    {
      "role": "user",
      "content": "こんにちは！最近暑いから甘いもの食べたくて"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "assess_interest",
        "description": "会話の応答と同時に、お客様の商品への興味度を判定する。...",
        "parameters": {
          "type": "object",
          "properties": {
            "interest_level": { "type": "integer", "minimum": 1, "maximum": 5 },
            "detected_preferences": { "type": "array", "items": { "type": "string" } },
            "response_text": { "type": "string" },
            "trigger_utterance": { "type": "string" }
          },
          "required": ["interest_level", "detected_preferences", "response_text", "trigger_utterance"]
        }
      }
    },
    {
      "type": "function",
      "function": {
        "name": "end_conversation",
        "description": "会話を終了する。お客様が会話を終えたい意思を示した時に使う。",
        "parameters": {
          "type": "object",
          "properties": {
            "closing_speech": { "type": "string" }
          },
          "required": ["closing_speech"]
        }
      }
    }
  ]
}
```

**レスポンス（ストリーミング）:**

LLMは通常の応答テキストと `assess_interest` の呼び出しを同時に返す。

```
// ストリームチャンク（テキスト部分）
data: {"choices":[{"delta":{"content":"暑い日が続きますよね！甘いもの、いいですね〜。どんな味がお好きですか？"}}]}

// ストリームチャンク（tool_calls 部分）
data: {"choices":[{"delta":{"tool_calls":[{"function":{"name":"assess_interest","arguments":"{\"interest_level\":3,\"detected_preferences\":[\"甘い\"],\"response_text\":\"甘いものに興味があるんですね！\",\"trigger_utterance\":\"甘いもの食べたくて\"}"}}]}}]}
```

**バックエンド処理:**
1. テキスト部分 → TTS変換 → `text_delta` / `text_done` でクライアントに送信
2. `assess_interest` の結果を受け取り:
   - `interest_level`（3）を `interest_threshold`（デフォルト: 3）と比較
   - `3 >= 3` → 閾値超過 → **パターンBに移行**

---

#### パターンA-2: 通常会話 + 興味判定（閾値未満）

**発生条件:** `assess_interest` の `interest_level` が閾値未満

**レスポンス例:**

```
// テキスト
data: {"choices":[{"delta":{"content":"こんにちは！今日はいいお天気ですね。何かお手伝いできることはありますか？"}}]}

// tool_calls
data: {"choices":[{"delta":{"tool_calls":[{"function":{"name":"assess_interest","arguments":"{\"interest_level\":1,\"detected_preferences\":[],\"response_text\":\"\",\"trigger_utterance\":\"こんにちは\"}"}}]}}]}
```

**バックエンド処理:**
1. テキスト → TTS → クライアントに送信（通常通り）
2. `interest_level`（1）< `interest_threshold`（3）→ ログ記録のみ、第2段階には移行しない

---

#### パターンB: 商品推薦（第2段階）

**発生条件:** パターンAで `interest_level >= interest_threshold` と判定された直後

バックエンドが以下を行った上で新たなLLMリクエストを発行する:
1. `assess_interest` の `response_text` をTTS変換してクライアントに送信
2. システムプロンプトに商品一覧を注入
3. `recommend_product` をツールに追加

**リクエスト:**

```json
{
  "model": "llama-3.3-70b-versatile",
  "stream": true,
  "messages": [
    {
      "role": "system",
      "content": "[Personal]\n私は「おためしちゃん」というアシスタントです。...\n\n## 興味判定ガイドライン\n...\n\n## 取扱商品一覧\n- h001: ハイチュウ ＜グレープ＞ [甘い,フルーティー,定番]\n  ジューシーな果汁感あふれる定番フレーバー\n- h002: ハイチュウ ＜ストロベリー＞ [甘酸っぱい,フルーティー,定番]\n  いちごの華やかな香りと甘酸っぱさ\n\n## 商品推薦ガイドライン\nお客様の好みに最も合う商品を1つ選び、recommend_product 関数で紹介してください。\n- 必ず1商品を選ぶこと\n- なぜその商品を薦めるのか理由を添える\n- introduction_speech は2文以内で簡潔に"
    },
    {
      "role": "user",
      "content": "こんにちは！最近暑いから甘いもの食べたくて"
    },
    {
      "role": "assistant",
      "content": "暑い日が続きますよね！甘いもの、いいですね〜。どんな味がお好きですか？"
    },
    {
      "role": "user",
      "content": "フルーティーな感じが好きかな"
    },
    {
      "role": "assistant",
      "content": "甘いものに興味があるんですね！"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "recommend_product",
        "description": "商品一覧の中から、お客様の好みに最も合う商品を1つ選んで紹介する。必ず1商品を選ぶこと。",
        "parameters": {
          "type": "object",
          "properties": {
            "product_id": {
              "type": "string",
              "enum": ["h001", "h002"],
              "description": "おすすめする商品のID（※ enum は商品マスタの id 一覧から都度動的に生成する）"
            },
            "introduction_speech": {
              "type": "string",
              "description": "商品を紹介するときの発話テキスト（2文以内）"
            },
            "reason": {
              "type": "string",
              "description": "推薦理由（会話から把握した好みとの関連）"
            }
          },
          "required": ["product_id", "introduction_speech", "reason"]
        }
      }
    },
  ],
  "tool_choice": {"type": "function", "function": {"name": "recommend_product"}}
}
```

**注意:** `tool_choice` で `recommend_product` を強制指定し、LLMが必ず商品を選ぶようにする。

**レスポンス（ストリーミング）:**

```
data: {"choices":[{"delta":{"tool_calls":[{"function":{"name":"recommend_product","arguments":"{\"product_id\":\"h001\",\"introduction_speech\":\"フルーティーな味がお好きなんですね！こちらのグレープ味のハイチュウはいかがですか？ジューシーな果汁感が楽しめますよ。\",\"reason\":\"甘くてフルーティーな味が好きとのことなので、果汁感あふれるグレープ味をおすすめします\"}"}}]}}]}
```

**バックエンド処理:**
1. `product_id`（"h001"）から商品情報を取得
2. `introduction_speech` をTTS変換
3. `product_recommendation` メッセージをクライアントに送信:
   ```json
   {
     "type": "product_recommendation",
     "transcript": "フルーティーな味がお好きなんですね！こちらのグレープ味のハイチュウはいかがですか？ジューシーな果汁感が楽しめますよ。",
     "audio_chunk": "<Base64>",
     "reason": "甘くてフルーティーな味が好きとのことなので、果汁感あふれるグレープ味をおすすめします",
     "products": [{ "product_id": "h001", "name": "ハイチュウ ＜グレープ＞", "..." : "..." }]
   }
   ```
4. 推薦ログを記録
5. `post_recommendation_behavior` に応じた後続処理

---

#### パターンC: 会話終了

**発生条件:** ユーザーが会話終了の意思を示した時

**レスポンス（ストリーミング）:**

```
data: {"choices":[{"delta":{"tool_calls":[{"function":{"name":"end_conversation","arguments":"{\"closing_speech\":\"お話できて楽しかったです！ハイチュウ、ぜひ試してみてくださいね。またお気軽にお声がけください！\"}"}}]}}]}
```

**バックエンド処理:**
1. `closing_speech` をTTS変換
2. 音声をクライアントに送信
3. `session_close` メッセージを送信:
   ```json
   {
     "type": "session_close",
     "reason": "conversation_ended",
     "message": "お話できて楽しかったです！ハイチュウ、ぜひ試してみてくださいね。またお気軽にお声がけください！"
   }
   ```

---

#### パターンD: 商品0件（機能OFF）

**発生条件:** `products` が空配列

**リクエスト:**

```json
{
  "model": "llama-3.3-70b-versatile",
  "stream": true,
  "messages": [
    {
      "role": "system",
      "content": "[Personal]\n私は「おためしちゃん」というアシスタントです。..."
    },
    {
      "role": "user",
      "content": "何かおすすめありますか？"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "end_conversation",
        "description": "...",
        "parameters": { "..." : "..." }
      }
    }
  ]
}
```

`assess_interest` も `recommend_product` も登録されない。興味判定ガイドラインもシステムプロンプトに含まれない。LLMは通常の会話応答のみを行う。

---

#### パターンE: シーケンシャルモードの関連商品紹介

**発生条件:** `recommendation_mode` が `sequential` で、パターンBで選定された商品に `related_product_ids` が設定されている時

パターンBの商品紹介完了後、バックエンドが `related_product_ids` の順に1商品ずつLLMリクエストを発行する。ユーザーの発話は挟まない。

**リクエスト（例: h001 紹介後、関連商品 h002 を紹介）:**

```json
{
  "model": "llama-3.3-70b-versatile",
  "stream": true,
  "messages": [
    {
      "role": "system",
      "content": "[Personal]\n私は「おためしちゃん」というアシスタントです。...\n\n## 取扱商品一覧\n- h001: ハイチュウ ＜グレープ＞ [甘い,フルーティー,定番]\n  ジューシーな果汁感あふれる定番フレーバー\n- h002: ハイチュウ ＜ストロベリー＞ [甘酸っぱい,フルーティー,定番]\n  いちごの華やかな香りと甘酸っぱさ\n\n## 商品紹介ガイドライン\n先ほど紹介した商品に関連する商品を続けて紹介してください。\n- 前の商品との関連性に触れながら自然につなげる\n- introduction_speech は2文以内で簡潔に"
    },
    {
      "role": "user",
      "content": "こんにちは！最近暑いから甘いもの食べたくて"
    },
    {
      "role": "assistant",
      "content": "暑い日が続きますよね！甘いもの、いいですね〜。"
    },
    {
      "role": "assistant",
      "content": "甘いものに興味があるんですね！"
    },
    {
      "role": "assistant",
      "content": "フルーティーな味がお好きなんですね！こちらのグレープ味のハイチュウはいかがですか？ジューシーな果汁感が楽しめますよ。"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "recommend_product",
        "description": "関連商品をお客様に紹介する。先に紹介した商品との関連性に触れながら自然につなげること。",
        "parameters": {
          "type": "object",
          "properties": {
            "product_id": {
              "type": "string",
              "enum": ["h002"],
              "description": "紹介する商品のID（※ バックエンドが次の関連商品IDのみをenumにセットする）"
            },
            "introduction_speech": {
              "type": "string",
              "description": "商品を紹介するときの発話テキスト（2文以内）"
            },
            "reason": {
              "type": "string",
              "description": "推薦理由（前の商品との関連性を踏まえた紹介の理由）"
            }
          },
          "required": ["product_id", "introduction_speech", "reason"]
        }
      }
    }
  ],
  "tool_choice": {"type": "function", "function": {"name": "recommend_product"}}
}
```

**ポイント:**
- `product_id` の `enum` にはバックエンドが次の関連商品IDのみをセットする
- `tool_choice` で `recommend_product` を強制指定
- 会話履歴には前の商品の `introduction_speech` が assistant メッセージとして含まれているため、LLMは前の紹介内容を踏まえて自然につなげられる
- `assess_interest` / `end_conversation` は登録しない（関連商品紹介中は割り込み不要）

**レスポンス（ストリーミング）:**

```
data: {"choices":[{"delta":{"tool_calls":[{"function":{"name":"recommend_product","arguments":"{\"product_id\":\"h002\",\"introduction_speech\":\"グレープがお好きでしたら、ストロベリー味もおすすめですよ！甘酸っぱいいちごの香りが楽しめます。\",\"reason\":\"同じフルーティー系のハイチュウで、グレープとの味比べも楽しい\"}"}}]}}]}
```

**バックエンド処理:**
1. `introduction_speech` をTTS変換
2. `product_recommendation` メッセージをクライアントに送信
3. 推薦ログを記録
4. `post_recommendation_behavior` に応じた後続処理
5. `related_product_ids` の次の商品があれば続けてパターンEを繰り返す
6. 全関連商品紹介済み → 通常会話ループに戻る

---

#### パターン一覧サマリ

`messages` 配列には `user` → `assistant` → `user` → `assistant` と交互にメッセージを格納する。以下のルールに従う。

**基本方針:** tool_calls/tool結果は履歴に含めず、ユーザーに実際に発話したテキストのみを `assistant` の `content` として残す。

| 元の処理 | 履歴への格納方法 |
|---|---|
| 通常のテキスト応答 | そのまま `{"role": "assistant", "content": "応答テキスト"}` |
| `assess_interest` の tool_calls / tool結果 | **含めない**（内部シグナルであり会話の文脈ではない） |
| `assess_interest` の `response_text`（「甘いものに興味があるんですね！」等） | `{"role": "assistant", "content": "甘いものに興味があるんですね！"}` |
| `recommend_product` の `introduction_speech` | `{"role": "assistant", "content": "こちらのグレープ味のハイチュウはいかがですか？"}` |
| `end_conversation` の `closing_speech` | 含めない（セッション終了のため後続リクエストなし） |

**理由:**
- LLMは `role` の切り替わりで「誰が何を言ったか」を認識する。1つのメッセージにまとめたり、roleを省略すると文脈理解と function calling の判断精度が低下する
- `assess_interest` の tool_calls を履歴に含めると、対応する `tool` ロールのメッセージも必須となり履歴構造が複雑化する上、トークンを浪費する
- 過去の興味判定結果をLLMに伝える必要はない。現在の会話文脈から都度判定できる

**具体例（パターンA → B の遷移時）:**

```json
"messages": [
  {"role": "system", "content": "...（第2段階のシステムプロンプト。商品一覧注入済み）"},
  {"role": "user", "content": "こんにちは！最近暑いから甘いもの食べたくて"},
  {"role": "assistant", "content": "暑い日が続きますよね！甘いもの、いいですね〜。どんな味がお好きですか？"},
  {"role": "user", "content": "フルーティーな感じが好きかな"},
  {"role": "assistant", "content": "フルーティーな味、いいですよね！"},
  {"role": "assistant", "content": "甘いものに興味があるんですね！"}
]
```

※ 最後の assistant メッセージは `assess_interest` の `response_text` から生成されたもの。tool_calls 自体は履歴に残さない。

---

#### パターン一覧サマリ

| パターン | 発生条件 | システムプロンプト | 登録ツール | tool_choice | 主な出力 |
|---|---|---|---|---|---|
| A | 毎回のユーザー発話（商品1件以上） | 基本 + 興味判定ガイドライン | assess_interest, end_conversation | auto | テキスト応答 + assess_interest |
| A-2 | Aと同じ（interest_level < 閾値） | 同上 | 同上 | auto | テキスト応答のみ（ログ記録） |
| B | interest_level >= 閾値 | 基本 + 興味判定 + 商品一覧 + 推薦ガイドライン | recommend_product | recommend_product 強制 | recommend_product 呼出し |
| C | ユーザーが終了意思を表示 | その時点のプロンプト | その時点のツール | auto | end_conversation 呼出し |
| D | 商品0件 | 基本のみ | end_conversation のみ | auto | テキスト応答のみ |
| E | シーケンシャルモードの関連商品紹介 | 基本 + 商品一覧 + 紹介ガイドライン | recommend_product（次の関連商品1つのみ） | recommend_product 強制 | recommend_product 呼出し |

---

## 3. 現状の実装

### 3.1 全体フロー

```
ユーザー発話 → STT → LLM（会話 + Function Calling判定）
                         │
                         ├─ 通常応答 → テキスト + TTS音声を返却
                         │
                         └─ recommend_product 呼出し
                              → 商品情報取得
                              → 紹介セリフをTTS変換
                              → product_recommendation メッセージ送信
                              → フロントエンドに商品カード表示
```

基本のフローは仕様通り実装されている。

### 3.2 商品データ

**ファイル:** `backend/internal/config/config.go`（デフォルト）、`backend/data/config.yaml`（上書き）

```go
type Product struct {
    ID          string `json:"id" yaml:"id"`
    Name        string `json:"name" yaml:"name"`
    Price       int    `json:"price" yaml:"price"`
    Description string `json:"description" yaml:"description"`
    ImageURL    string `json:"image_url" yaml:"image_url"`
}
```

- デフォルト設定に3商品が定義されている（ハイチュウ グレープ/ストロベリー/グリーンアップル）
- **`config.yaml` の `products: []` で上書きされるため、現在は商品が0件で機能が無効化されている**
- `category`、`tags` フィールドは存在しない

### 3.3 LLM Function Calling

**ファイル:** `backend/internal/openai/chat.go` — `buildTools()`（186-241行）

| ツール名 | 条件 | パラメータ |
|---|---|---|
| `recommend_product` | `len(products) > 0` の場合のみ登録 | `product_ids: string[]`、`introduction_speech: string` |
| `end_conversation` | 常に登録 | `closing_speech: string` |

- `recommend_product` に `reason` パラメータは存在しない
- ツール定義の `description` に推薦タイミングの制約は記述されている

### 3.4 システムプロンプト

**ファイル:** `backend/internal/openai/chat.go` — `buildSystemPrompt()`（243-259行）

商品が1件以上ある場合に追記される内容:

```
## 取扱商品一覧
- h001: ハイチュウ ＜グレープ＞（140円）— ジューシーな果汁感あふれる定番フレーバー

## 関数の使い方
- recommend_product: お客様が商品について具体的に質問したり、
  「おすすめを教えて」「何がある？」など明確に商品紹介を求めた場合にのみ使う。
  雑談や挨拶では絶対に使わないこと。
- end_conversation: お客様が「さようなら」など会話を終えたい意思を明確に示した時に使う。
```

- 推薦タイミングのガイドラインが簡素（「明確に求めた場合にのみ」としか書いていない）
- `category`/`tags` 情報は含まれていない

### 3.5 バックエンド処理

**ファイル:** `backend/internal/session/manager.go`

#### recommend_product ハンドリング（719-758行）

1. `RecommendProductArgs` をパース（`product_ids`、`introduction_speech`）
2. `catalog.GetProducts(args.ProductIDs)` で商品情報を取得
3. `introduction_speech` をTTSで音声合成
4. `product_recommendation` メッセージを送信（商品情報 + 音声）
5. 状態を `StateListening` に遷移、無音タイマーを再開
6. `handlePostRecommendation()` を実行

- `reason` の取得・送信は行っていない
- 構造化ログへの推薦イベント記録はない

#### end_conversation ハンドリング（760-767行）

```go
case "end_conversation":
    var args openai.EndConversationArgs
    // ... パース ...
    m.initiateClose("conversation_ended", "")  // ← closing_speech が無視されている
```

- `closingSpeech()` 関数（813-824行）がハードコードされた定型文を返す
- LLMが生成した `closing_speech` は使用されない

#### 推薦後の動作（770-788行）

| 設定値 | 動作 |
|---|---|
| `return_to_conversation` | 何もせず通常会話に戻る |
| `ask_interest` | 固定テキスト「ご紹介した商品はいかがですか？」を送信 |

- シーケンシャルモード: `seqIndex++` するだけで実際の推薦ロジックは未実装

### 3.6 WebSocket プロトコル

#### セッション設定

```json
{
  "recommendation_mode": "ai_driven",
  "sequential_items": [],
  "sequential_interval_sec": 30,
  "post_recommendation_behavior": "return_to_conversation"
}
```

- `sequential_items`、`sequential_interval_sec` は定義されているが機能しない

#### 商品推薦メッセージ

```json
{
  "type": "product_recommendation",
  "transcript": "こちらのグレープ味がおすすめですよ！",
  "audio_chunk": "<Base64 音声>",
  "products": [
    {
      "product_id": "h001",
      "name": "ハイチュウ ＜グレープ＞",
      "price": 140,
      "description": "ジューシーな果汁感あふれる定番フレーバー",
      "image_url": "/images/h001.png"
    }
  ]
}
```

- `reason` フィールドは存在しない
- `category`、`tags` は `ProductInfo` に含まれていない

### 3.7 フロントエンド表示

#### ProductCard.vue

- グラデーションヘッダー（商品名キーワードで色決定、マッチしなければIDハッシュ）
- 商品名、価格（JPYフォーマット）、説明文
- 「詳細」ボタン → **`alert()` でテキスト表示のみ**
- **商品画像は表示されない**（`image_url` フィールドはあるが未使用）

#### ProductCardList.vue

- 1商品: 中央寄せ
- 複数商品: レスポンシブグリッド（1列 → sm:2列 → lg:3列）
- **推薦理由の表示はない**

### 3.8 設定UI

**ファイル:** `frontend/components/SessionConfigPanel.vue`

折り畳みパネル内にラジオボタンで設定:
- レコメンドモード（AI駆動 / シーケンシャル）
- シーケンシャルモード時: 商品リストJSON入力、推薦間隔
- 推薦後の動作（会話に戻る / 興味を確認する）

---

## 4. 差異分析

### 4.1 差異一覧

| # | 項目 | あるべき仕様 | 現状 | 差異の種別 |
|---|---|---|---|---|
| G1 | 商品データ | 商品が0件の場合は機能全体をOFF（assess_interest登録なし・第2段階なし・interest_promptのシステムプロンプト注入なし） | `config.yaml` で `products: []` → 機能OFF状態 | **仕様通り** |
| G2 | 商品データ構造 | `tags` フィールドを持つ | `id`、`name`、`description`、`image_url` のみ | 機能不足 |
| G3 | 推薦理由 | `reason` パラメータでLLMから取得し、UIに表示 | `reason` パラメータが存在しない | 機能不足 |
| G4 | システムプロンプト | 2段階推薦の詳細ガイドライン | 「明確に求めた場合にのみ使う」の1行のみ | 品質不足 |
| G5 | end_conversation | LLM生成の `closing_speech` を使用 | `closing_speech` がパースされるが無視される。定型文を使用 | **実装バグ** |
| G6 | 商品画像 | `image_url` がある場合は実画像を表示 | グラデーション背景のみ。`image_url` は未使用 | 機能不足 |
| G7 | 商品詳細 | モーダルで全情報を表示 | `alert()` でテキスト表示のみ | 品質不足 |
| G8 | 推薦理由のUI表示 | カードリスト上部に推薦理由を表示 | 表示なし | 機能不足（G3に依存） |
| G9 | 推薦ログ | 興味判定・推薦の両方を構造化ログに記録 | ログなし | 機能不足 |
| G10 | シーケンシャルモード | 設定画面で商品リストと間隔を設定し、タイマーで順番に紹介する | UIとプロトコル定義は存在するがロジック未実装 | **機能不足** |
| G11 | プロトコル: ProductInfo | `tags` を含む | 含まない | 機能不足（G2に依存） |
| G12 | プロトコル: ProductRecommendation | `reason` を含む | 含まない | 機能不足（G3に依存） |
| G13 | 興味判定（assess_interest） | 推薦前に興味判定を行い、interest_levelに応じてフローを分岐 | 存在しない。LLMが直接 recommend_product を呼ぶ | **機能不足** |

### 4.2 差異の分類

#### 仕様通り（現状の実装が正しい）

| # | 内容 | 備考 |
|---|---|---|
| G1 | 商品データが空 → 機能全体OFF | 商品が0件の場合、assess_interest・recommend_productともに登録せず、interest_promptもシステムプロンプトに注入しない。商品データを登録すれば機能が有効になる。 |

#### 実装バグ（仕様は正しいが実装が壊れている）

| # | 内容 | 影響度 | 修正コスト |
|---|---|---|---|
| G5 | LLM生成の closing_speech が無視される | 中 — 定型文でも動作はするが不自然 | 低（1行の引数変更） |

#### 機能不足（仕様に対して未実装の機能）

| # | 内容 | 影響度 | 修正コスト |
|---|---|---|---|
| G2 | tags がない | 低〜中 — LLMの推薦精度に影響 | 低（構造体とconfigに追加） |
| G3 | 推薦理由の取得がない | 中 — ユーザー体験とログ分析に影響 | 低（Function定義にパラメータ追加） |
| G6 | 商品画像が表示されない | 中 — 視覚的な訴求力が弱い | 低（img タグ追加 + フォールバック） |
| G9 | 推薦ログがない | 低 — 運用・分析ができない | 低（slog.Info 追加） |
| G13 | 興味判定ステップがない | **高** — 推薦の精度・自然さに直結 | 中（新規Function定義 + バックエンドハンドラ + 誘導ロジック） |

#### 品質不足（動くが品質が低い）

| # | 内容 | 影響度 | 修正コスト |
|---|---|---|---|
| G4 | システムプロンプトが簡素 | 中 — LLMの推薦判断が粗い | 低（プロンプト文面の改善） |
| G7 | 商品詳細が alert() | 低〜中 — UX が悪い | 中（モーダルコンポーネント新規作成） |

#### 未完成（仕様は決定、実装が必要）

| # | 内容 | 影響度 | 修正コスト |
|---|---|---|---|
| G10 | シーケンシャルモード（UIとプロトコル定義は存在するがロジック未実装。関連商品紐づけ・連続紹介が必要） | 中 — 店頭デモ等で必要 | 中（`related_product_ids` 追加 + 関連商品連続紹介ロジック + 設定画面の紐づけUI） |

### 4.3 推奨対応優先度

```
Phase 1（コアロジック）:   G13 → G5 → G4 → G3 → G9
Phase 3（UI改善）:        G6 → G7 → G8 → G2 → G11 → G12
Phase 4（方針決定）:      G10
```

---

## 5. ファイル変更一覧

| ファイル | 関連差異 | 変更内容 |
|---|---|---|
| `backend/data/config.yaml` | G2 | `tags` フィールド追加（商品データ自体は運用時に登録） |
| `backend/internal/config/config.go` | G2 | `Product` 構造体に `Tags` フィールド追加。`Price` フィールド削除 |
| `backend/internal/openai/chat.go` | G3, G4, G13 | `assess_interest` Function追加。`recommend_product` に `reason` 追加。システムプロンプトの2段階ガイドライン改善 |
| `backend/internal/openai/models.go` | G3, G13 | `AssessInterestArgs` 構造体追加。`RecommendProductArgs` に `Reason` フィールド追加 |
| `backend/internal/session/manager.go` | G5, G9, G3, G13 | `assess_interest` ハンドラ追加（interest_levelに応じた誘導メッセージ注入）。`closing_speech` を使用。推薦ログ追加。`reason` をプロトコルに含める |
| `backend/internal/protocol/messages.go` | G11, G12 | `ProductInfo` に `Tags` 追加、`Price` 削除。`ProductRecommendation` に `Reason` 追加 |
| `frontend/types/protocol.ts` | G11, G12 | 型定義の同期 |
| `frontend/components/ProductCard.vue` | G6, G7 | 商品画像表示。詳細モーダル化 |
| `frontend/components/ProductCardList.vue` | G8 | 推薦理由の表示 |
| `frontend/components/ProductDetailModal.vue` | G7 | **新規作成** — 商品詳細モーダル |

---

## 6. セッション状態遷移とオーディオパイプライン

### 6.1 セッション状態（SessionState）

フロントエンド（`useSession.ts`）が管理するセッション状態。UIの表示切替とマイク制御に使用する。

| 状態 | 意味 | マイク（pausedフラグ） |
|---|---|---|
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

### 6.2 状態遷移図

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

### 6.3 正常フロー

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

### 6.4 バージイン（割り込み）

AI発話中にユーザーが割り込む機能。音声割り込みとテキスト割り込みの2種類がある。

#### 音声割り込み

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

#### テキスト割り込み

```
1. sendText() 内で ai_speaking / processing / 音声再生中を検知
2. audioPlayback.stopAndClear()
3. state = 'barge_in'
4. テキスト送信 → state = 'processing'
```

#### バックエンドの割り込み処理（interruptIfBusy）

**ファイル:** `backend/internal/session/manager.go`

```go
func (m *Manager) interruptIfBusy() {
    currentState := m.state.Current()
    if currentState != StateAISpeaking && currentState != StateProcessing {
        return
    }
    // LLM/TTSパイプラインのキャンセル
    if m.cancelCurrent != nil {
        m.cancelCurrent()
    }
    // フロントエンドに音声バッファクリアを指示
    m.send(TypeClearAudioBuffer, ClearAudioBuffer{Reason: "barge_in"})
    m.state.Transition(StateListening)
}
```

- `handleAudioSpeech()` と `handleAudioChunk()` の冒頭で呼ばれる
- `Processing` または `AISpeaking` 状態の場合のみ発動
- `context.Cancel()` により進行中のLLMストリーミングとTTS合成を即座に中断

### 6.5 音声入力パイプライン

**ファイル:** `frontend/composables/useVoiceInput.ts`

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
|---|---|---|
| `SILENCE_THRESHOLD` | 0.015 | RMS閾値。これを超えると発話と判定 |
| `SILENCE_FRAMES_REQUIRED` | 30 | 発話終了と判定する連続無音フレーム数（約80ms） |
| `MIN_SPEECH_FRAMES` | 5 | 最低発話フレーム数（約13ms）。これ未満は破棄 |

- 音量ベースの簡易VAD。文単位や意味的な区切りではない
- `echoCancellation: true` と RNNoise の組み合わせでAI再生音のフィードバックを抑制

#### 音声エンコード

- RNNoise処理済みのPCMデータを `OfflineAudioContext` → `MediaRecorder`（`audio/webm;codecs=opus`、32kbps）でエンコード
- エンコードは非同期（fire-and-forget）。エンコード完了後に `onSpeechEnd` コールバックが呼ばれる

### 6.6 音声再生パイプライン

**ファイル:** `frontend/composables/useAudioPlayback.ts`

```
audio_stream / text_done.audio_chunk（Base64）
  → decodeAudioData → AudioBuffer キュー（最大50）
  → 順次再生（source.onended → playNext）
```

| 機能 | メソッド | 説明 |
|---|---|---|
| 再生 | `playAudio(base64)` | デコードしてキューに追加、未再生なら再生開始 |
| 割り込み停止 | `stopAndClear()` | キュー全クリア＋現在の再生を即停止 |
| 完了待ち | `waitUntilDone()` | キュー＋再生＋デコード待ちがすべて完了するまで待機 |
| 音量制御 | `setVolume(val)` / `toggleMute()` | `GainNode` で音量調整 |

### 6.7 TTS区切り単位

**ファイル:** `backend/internal/textbuf/buffer.go`

LLMのストリーミング出力を句読点で区切り、文単位でTTSに投入する。

| ルール | 条件 | 動作 |
|---|---|---|
| 句読点区切り | `。` `、` `！` `？` `…` を検出 | その位置で分割してTTSに送信 |
| 文字数上限 | 100文字を超過 | 句読点がなくても強制分割 |
| タイムアウト | 1秒間新しいトークンが来ない | バッファ内の残りを強制送信 |
| LLM完了 | `text_done` イベント | `Flush()` で残りを送信 |

句読点で細かく区切ることで、音声再生開始までのレイテンシを最小化する。

### 6.8 STTエンジン

設定画面でエンジンを切り替え可能。

| エンジン | 設定値 | 説明 |
|---|---|---|
| faster-whisper | `whisper` | ローカルのfaster-whisperサーバー（CTranslate2ベース）。Opus/WebM対応 |
| Groq | `groq` | Groq APIを使用。モデル選択可能（whisper-large-v3-turbo等） |

#### 音声フォーマット

- フロントエンド → バックエンド: Opus/WebM（Base64）
- バックエンド → STT: Opus/WebMバイナリをそのまま送信（WAV変換不要）
- faster-whisper: 内部でffmpegにより自動変換
- Groq API: Opus/WebMネイティブ対応
