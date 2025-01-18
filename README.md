Build:

```
go build
```

Docker:

```
docker build --platform linux/amd64 -t hurl-service .

docker run -p 8080:8080 \
  -e API_TOKEN=123 \
  -v ./results:/hurl_results \
  hurl-service
```
