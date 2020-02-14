FROM golang

WORKDIR /src
COPY . .

RUN go build -v -a \
	-tags "netgo osusergo" \
	-ldflags '-extldflags "-static"' \
	-o /usr/local/concourse/bin/baggageclaim \
	./cmd/baggageclaim
