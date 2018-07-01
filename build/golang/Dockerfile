FROM golang:1.12.1-stretch
RUN apt-get update && \
    apt-get install -y \
      curl \
      file \
      git \
      jq \
      libprotobuf-dev \
      make \
      protobuf-compiler \
      python3 \
      python3-pip \
      python-requests \
      shellcheck \
      unzip && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
RUN pip3 install \
      attrs==18.1.0 \
      pyhcl \
      yapf==0.16.2 \
      flake8==3.3.0 \
      pyyaml==3.13 \
      awscli==1.15.69 \
      git+git://github.com/weaveworks/grafanalib.git@v0.5.3
RUN curl -fsSLo shfmt https://github.com/mvdan/sh/releases/download/v1.3.0/shfmt_v1.3.0_linux_amd64 && \
	echo "b1925c2c405458811f0c227266402cf1868b4de529f114722c2e3a5af4ac7bb2  shfmt" | sha256sum -c && \
	chmod +x shfmt && \
	mv shfmt /usr/bin
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
		golang.org/x/lint/golint \
		github.com/golang/protobuf/protoc-gen-go \
		github.com/kisielk/errcheck \
		github.com/mjibson/esc \
		rm -rf /go/pkg /go/src
# Pin promtool to a specific version
RUN go get -d -tags netgo github.com/prometheus/prometheus/cmd/promtool && \
	cd /go/src/github.com/prometheus/prometheus && \
	git checkout v2.3.2 && \
	go install -tags netgo github.com/prometheus/prometheus/cmd/promtool && \
	rm -rf /go/pkg /go/src
RUN mkdir protoc && \
	cd protoc && \
	curl -O -L https://github.com/google/protobuf/releases/download/v3.1.0/protoc-3.1.0-linux-x86_64.zip && \
	unzip protoc-3.1.0-linux-x86_64.zip && \
	cp bin/protoc /usr/bin/ && \
	chmod o+x /usr/bin/protoc && \
	cd .. && \
	rm -rf protoc
RUN mkdir -p /var/run/secrets/kubernetes.io/serviceaccount && \
		touch /var/run/secrets/kubernetes.io/serviceaccount/token
COPY build.sh /
ENTRYPOINT ["/build.sh"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="golang" \
      org.opencontainers.image.source="https://github.com/weaveworks/build-tools/tree/master/build/golang" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
