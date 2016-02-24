go get github.com/naoina/toml
go get github.com/blackbeans/turbo
go get github.com/blackbeans/log4go
go get gopkg.in/redis.v3 		#只用来测试

go build git.wemomo.com/bibi/go-moa/core
go build git.wemomo.com/bibi/go-moa/lb
go build git.wemomo.com/bibi/go-moa/proxy
go build git.wemomo.com/bibi/go-moa/protocol



go install git.wemomo.com/bibi/go-moa/core
go install git.wemomo.com/bibi/go-moa/lb
go install git.wemomo.com/bibi/go-moa/proxy
go install git.wemomo.com/bibi/go-moa/protocol
