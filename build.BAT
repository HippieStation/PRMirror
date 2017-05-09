@echo off
echo Ready to build
pause
mkdir %UserProfile%\go\src\github.com\google\
mkdir %UserProfile%\go\src\github.com\boltdb\
mkdir %UserProfile%\go\src\github.com\op\
mkdir %UserProfile%\go\src\golang.org\x\
cd %UserProfile%\go\src\github.com\google
git clone --no-hardlinks https://github.com/google/go-github.git
git clone --no-hardlinks https://github.com/google/go-querystring.git
cd %UserProfile%\go\src\github.com\boltdb
git clone --no-hardlinks https://github.com/boltdb/bolt.git
cd %UserProfile%\go\src\github.com\op
git clone --no-hardlinks https://github.com/op/go-logging.git
cd %UserProfile%\go\src\golang.org\x\
git clone --no-hardlinks https://github.com/golang/oauth2.git
git clone --no-hardlinks https://github.com/golang/net.git
cd %~dp0
go build
pause