all: narval-arm narval-x64 narval.exe

narval-arm: *.go */*.go
	GOARCH=arm GOOS=linux go build -o narval-arm

narval-x64: *.go */*.go
	GOARCH=amd64 GOOS=linux go build -o narval-x64

narval.exe: *.go */*.go
	GOARCH=amd64 GOOS=windows go build -o narval.exe