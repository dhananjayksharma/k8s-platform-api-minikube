APP=k8s-platform-api-minikube
SVCAPP=k8s-platform-svc-minikube 
NS=platform-demo
PROFILE=-p gpu-cpu-lab

.PHONY: deps test build image image-all list deploy status url port-forward clean

deps:
	go mod tidy

test:
	go test ./...

build:
	go build ./cmd/api

# Build only on the primary Minikube node
image:
	minikube $(PROFILE) image build \
		-t $(APP):local .

# Build/cache image on ALL Minikube nodes
image-all:
	minikube $(PROFILE) image build \
		--all \
		-t $(APP):local .

list:
	minikube $(PROFILE) image ls --format=table

deploy:
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/postgres.yaml
	kubectl rollout status deployment/postgres -n $(NS) --timeout=120s
	kubectl apply -f k8s/api.yaml
	kubectl rollout status deployment/$(APP) -n $(NS) --timeout=120s

status:
	kubectl get pods -n $(NS) -o wide

url:
	minikube $(PROFILE) service $(SVCAPP) -n $(NS) --url

port-forward:
	kubectl port-forward -n $(NS) svc/$(SVCAPP) 8080:80

clean:
	kubectl delete namespace $(NS) --ignore-not-found