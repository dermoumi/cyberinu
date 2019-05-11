FROM golang:1.12

WORKDIR $GOPATH/src/github.com/dermoumi/cyberinu

COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD tail -f /dev/null
