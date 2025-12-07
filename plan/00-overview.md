# vrclog-go プロジェクト概要

## プロジェクトの目的

VRChatのPC版（Windows）のログファイルを解析・監視してイベントに変換するためのGoライブラリ＆CLI。

## 主な機能

1. **ログ解析**: VRChatのログファイルから構造化されたイベントを抽出
2. **リアルタイム監視**: ログファイルの変更を監視し、新しいイベントを即座に検出
3. **CLI提供**: コマンドラインから簡単に利用可能

## ターゲットユーザー

- VRChat関連ツールの開発者
- VRChatのログを分析したいユーザー
- Join通知ツール、履歴ビューアなどを構築したい開発者

## スコープ

### このリポジトリでやること

- ログファイルのパース（1行→構造化イベント）
- ログファイルのリアルタイム監視
- イベントをチャネル経由で提供するAPI
- CLI（標準出力にイベントを流す）

### このリポジトリでやらないこと

- データベースへの保存
- Web UI
- スマホ通知
- トレイアイコン
- ネットワークサーバー機能

## 技術スタック

- **言語**: Go 1.21+
- **CLI Framework**: spf13/cobra
- **File Tailing**: nxadm/tail
- **テスト**: 標準ライブラリのみ（testify不使用）

## リポジトリ情報

- **Organization**: github.com/vrclog
- **Repository**: github.com/vrclog/vrclog-go
- **対象OS**: Windows 11（VRChat PC版が動作する環境）

## 関連プロジェクト（参考）

| プロジェクト | 言語 | 特徴 |
|-------------|------|------|
| [VRCX](https://github.com/vrcx-team/VRCX) | C# | 最も活発、包括的なVRChatツール |
| [vrc-tail](https://github.com/Narazaka/vrc-tail) | - | ログファイル選択アルゴリズム |
| [XSOverlay-VRChat-Parser](https://github.com/nnaaa-vr/XSOverlay-VRChat-Parser) | C# | XSOverlay連携 |
| [VRChat-Log-Monitor](https://github.com/Kavex/VRChat-Log-Monitor) | Python | Discord連携 |
| [vrchat-log-rs](https://github.com/sksat/vrchat-log-rs) | Rust | Rustでの実装例 |
