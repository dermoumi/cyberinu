FROM golang:1.12

ENV SLACK_TOKEN=

WORKDIR $GOPATH/src/github.com/dermoumi/cyberinu

COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD cyberinu
