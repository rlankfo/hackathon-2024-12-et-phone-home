Instructions for local setup.

# build docker image
```
docker build -t otelcol-hackathon:latest .
```

# create k3d cluster
```
k3d cluster create hackathon-2024-12-et-phone-home
```

# add the image to the k3d cluster
```
k3d image import otelcol-hackathon:latest -c hackathon-2024-12-et-phone-home
```

# install otel-operator (requires cert manager)
```
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
kubectl wait --for=condition=Ready pods -n cert-manager --all
helm install otel-operator open-telemetry/opentelemetry-operator \
	--set manager.collectorImage.repository=otel/opentelemetry-collector \
	--set manager.collectorImage.tag=0.114.0
```

# install the otel demo
```
helm install otel-demo open-telemetry/opentelemetry-demo -f otel-demo-values.yaml
```

# patch otel demo service deployments with sidecar annotation
```
kubectl patch deployment otel-demo-accountingservice otel-demo-adservice otel-demo-cartservice otel-demo-checkoutservice otel-demo-currencyservice otel-demo-emailservice otel-demo-frauddetectionservice otel-demo-frontend otel-demo-frontendproxy otel-demo-paymentservice otel-demo-productcatalogservice otel-demo-quoteservice otel-demo-recommendationservice otel-demo-shippingservice \
	--type=json \
	-p='[{"op":"add","path":"/spec/template/metadata/annotations","value":{"sidecar.opentelemetry.io/inject":"sidecar-collector","opentelemetry.io/inject": "true"}}]'
```


# deploy opentelemetry collector sidecars
```
kubectl apply -f sidecar.yaml
```

# restart to inject sidecars (not sure if this is needed or my k3d api server is just crashing?)
```
kubectl rollout restart deployment otel-demo-accountingservice otel-demo-adservice otel-demo-cartservice otel-demo-checkoutservice otel-demo-currencyservice otel-demo-emailservice otel-demo-frauddetectionservice otel-demo-frontend otel-demo-frontendproxy otel-demo-paymentservice otel-demo-productcatalogservice otel-demo-quoteservice otel-demo-recommendationservice otel-demo-shippingservice
```
