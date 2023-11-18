CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o meteor_linux_amd64 main.go
#CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o meteor_windows_amd64.exe main.go
#CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o meteor_darwin_amd64 main.go
#CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o meteor_darwin_arm64 main.go

upx meteor_linux_amd64
#upx meteor_windows_amd64.exe
#upx meteor_darwin_amd64
#upx meteor_darwin_arm64