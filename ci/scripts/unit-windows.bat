set PATH=C:\Go\bin;C:\Program Files\Git\cmd;%PATH%

set GOPATH=%CD%\gopath
set PATH=%CD%\gopath\bin;%PATH%

cd .\baggageclaim

go mod download

go install github.com/onsi/ginkgo/ginkgo

ginkgo -r -p
