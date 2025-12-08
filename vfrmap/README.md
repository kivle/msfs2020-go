# msfs2020-go/vfrmap

lightweight data server using msfs2020-go/simconnect that streams simulator state over websockets. a minimal http endpoint is exposed for health/info; bring your own client ui.

## install

* download latest release zip [here](https://github.com/lian/msfs2020-go/releases)
* unzip `vfrmap-win64.zip`

## run
* run `vfrmap.exe`
* connect your websocket client to `ws://localhost:9000/ws`

## arguments

* `-v` show program version
* `-verbose` verbose output
* `-disable-teleport` disables teleport

## compile

`GOOS=windows GOARCH=amd64 go build github.com/lian/msfs2020-go/vfrmap` or see [build-vfrmap.sh](https://github.com/lian/msfs2020-go/blob/master/build-vfrmap.sh)

## Why does my virus-scanning software think this program is infected?

From official golang website https://golang.org/doc/faq#virus

"This is a common occurrence, especially on Windows machines, and is almost always a false positive. Commercial virus scanning programs are often confused by the structure of Go binaries, which they don't see as often as those compiled from other languages."
