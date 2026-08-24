APP=k8s-platform-api-minikube
NS=platform-demo

.PHONY: deps test build image deploy status url port-forward clean

deps:
	go mod tidy

test:
	go test ./...

build:
	go build ./cmd/api

image:
	minikube image build -t $(APP):local .

deploy:
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/postgres.yaml
	kubectl rollout status deployment/postgres -n $(NS) --timeout=120s
	kubectl apply -f k8s/api.yaml
	kubectl rollout status deployment/k8s-platform-api-minikube -n $(NS) --timeout=120s

status:
	kubectl get all -n $(NS)

url:
	minikube service k8s-platform-api-minikube -n $(NS) --url

port-forward:
	kubectl port-forward -n $(NS) svc/k8s-platform-api-minikube 8080:80

clean:
	kubectl delete namespace $(NS) --ignore-not-found
