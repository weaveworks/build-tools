FROM golang:1.8.0-stretch
RUN apt-get update && apt-get install -y python-requests python-yaml file jq unzip protobuf-compiler libprotobuf-dev && \
	rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
RUN go clean -i net && \
	go install -tags netgo std && \
	go install -race -tags netgo std
RUN go get -tags netgo \
		github.com/FiloSottile/gvt \
		github.com/client9/misspell/cmd/misspell \
		github.com/fatih/hclfmt \
		github.com/fzipp/gocyclo \
		github.com/gogo/protobuf/gogoproto \
		github.com/gogo/protobuf/protoc-gen-gogoslick \
		github.com/golang/dep/... \
		github.com/golang/lint/golint \
		github.com/golang/protobuf/protoc-gen-go \
		github.com/kisielk/errcheck \
		github.com/mjibson/esc \
		github.com/mvdan/sh/cmd/shfmt && \
		rm -rf /go/pkg /go/src
RUN mkdir protoc && \
	cd protoc && \
	curl -O -L https://github.com/google/protobuf/releases/download/v3.1.0/protoc-3.1.0-linux-x86_64.zip && \
	unzip protoc-3.1.0-linux-x86_64.zip && \
	cp bin/protoc /usr/bin/ && \
	chmod o+x /usr/bin/protoc && \
	cd .. && \
	rm -rf protoc
COPY build.sh /
ENTRYPOINT ["/build.sh"]
