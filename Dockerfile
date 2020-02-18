FROM golang AS base

	WORKDIR /src
	COPY . .

	RUN go build -v -a \
		-tags "netgo osusergo" \
		-ldflags '-extldflags "-static"' \
		-o /usr/local/concourse/bin/baggageclaim \
		./cmd/baggageclaim

	RUN go build -v -a \
		-tags "netgo osusergo" \
		-ldflags '-extldflags "-static"' \
		-o /usr/local/concourse/bin/beltloader \
		./cmd/beltloader


FROM ubuntu

	ENV PATH /usr/local/concourse/bin:$PATH
	COPY --from=base /usr/local/concourse/bin /usr/local/concourse/bin
