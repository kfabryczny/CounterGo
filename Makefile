#

NAME = Counter
PACKAGE = CounterGo
VERSION = v2.0.1

all:
	GOOS=linux   GOARCH=amd64 go build -o $(NAME)-$(VERSION)-linux-64       $(PACKAGE)
	GOOS=windows GOARCH=amd64 go build -o $(NAME)-$(VERSION)-windows-64.exe $(PACKAGE)
