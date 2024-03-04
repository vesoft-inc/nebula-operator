TAG=v1
IMAGE=vesoft/startup-script

.PHONY: build push

build:
	docker build --pull --network host -t $(IMAGE):$(TAG) .

push: build
	docker push $(IMAGE):$(TAG)