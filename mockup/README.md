# SSNP Mockup

依存ゼロの静的モックアップです。ローカル確認は以下で十分です。

```sh
cd path/to/ssnp_sip/mockup
python3 -m http.server 8000
```

ブラウザで `http://localhost:8000` を開いて確認してください。

構成:

- `index.html`: モックアップ本体
- `styles.css`: レイアウトと配色
- `app.js`: ランキング表のダミーデータ描画と簡易フィルタ
