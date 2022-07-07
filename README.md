# libgo-http

## conv/header

生HTTP HeaderをJSON形式などに変換します。

jqのようなコマンドと組み合わせれば、シェルで利用可能になります。

### 使用方法

#### プログラム呼び出し例

プログラムで利用する場合はconv/header.goを使用します。

```golang
import (
	"github.com/ngv-jp/libgo-http/conv"
)
（中略）
	c := conv.NewHttpHeaderConverter()
	r, e := c.Output()
```

#### コマンド実行例

コマンドで実行する場合はconv/main/header.goを使用します。
パイプで繋げれば、標準出力を入力データにすることが可能です。

```bash
curl -I https://example.com | go run conv/main/header.go
```

デフォルトではJSON形式で出力されます。

```json
{"acceptRanges":"bytes","age":"197337","cacheControl":"max-age=604800","code":"200","contentEncoding":"gzip","contentLength":"648","contentType":"text/html; charset=UTF-8","date":"Thu, 07 Jul 2022 14:50:20 GMT","etag":"\"3147526947\"","expires":"Thu, 14 Jul 2022 14:50:20 GMT","lastModified":"Thu, 17 Oct 2019 07:18:26 GMT","server":"ECS (sec/973B)","xCache":"HIT"}
```
