# 出退勤状態共有

1. 勤務状況登録
2. 出勤・退勤履歴保存
3. 状況共有・確認(要コマンド)

```go
// 以下定数に設定した値(投稿)があればステータスの更新が行われます
// LINE Developersで設定できます。
const (
	_                  TypeStatus = iota
	TypeStatusActive              // 出勤
	TypeStatusInactive            // 退勤
	TypeStatusBreak               // 休憩
	TypeStatusOut                 // 外回り/会議/打ち合わせ
	TypeStatusFocus               // 重要/業務集中/運転中
	TypeStatusDeskwork            // 事務作業
)
```

# Usage
- Google cloud run
- Google cloud firestore
- Google cloud storage
- LINE Developers

# CONST
- PROJECTID: Google cloud project id
- ACCESSTOKEN: LINEアクセストークン
- CHANNELSECRET: LINEチャンネルシークレットトークン
- BUCKETNAME: ファイル保存バケツ名


# DEPLOY

```sh
$ gcloud run deploy --set-env-vars "PROJECTID=,ACCESSTOKEN=,CHANNELSECRET=,BUCKETNAME="
```

## TODO
- 出勤・退勤履歴出力
- 時間外労働・備考入力


## Options
- 出退勤厳格化(社内QRの当日出力かつ読み込み必須)
- AI Chat
- GPS自動出退勤及びMapURL出力