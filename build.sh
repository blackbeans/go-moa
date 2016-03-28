go get github.com/naoina/toml
go get github.com/blackbeans/turbo
go get github.com/blackbeans/log4go
go get gopkg.in/redis.v3 	
go get github.com/go-errors/errors

go build github.com/blackbeans/go-moa/core
go build github.com/blackbeans/go-moa/lb
go build github.com/blackbeans/go-moa/proxy
go build github.com/blackbeans/go-moa/protocol
go build github.com/blackbeans/go-moa/log4moa


go install github.com/blackbeans/go-moa/core
go install github.com/blackbeans/go-moa/lb
go install github.com/blackbeans/go-moa/proxy
go install github.com/blackbeans/go-moa/protocol
go install github.com/blackbeans/go-moa/log4moa
