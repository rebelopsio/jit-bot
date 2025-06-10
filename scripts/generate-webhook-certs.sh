#!/bin/bash

# Script to generate self-signed certificates for webhook server
# Use this when cert-manager is not available

set -e

NAMESPACE=${NAMESPACE:-jit-system}
SERVICE_NAME=${SERVICE_NAME:-jit-operator-webhook-service}
SECRET_NAME=${SECRET_NAME:-jit-operator-webhook-certs}

echo "Generating webhook certificates for ${SERVICE_NAME} in namespace ${NAMESPACE}"

# Create temporary directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Generate CA private key
openssl genrsa -out ca.key 2048

# Generate CA certificate
openssl req -new -x509 -days 365 -key ca.key -out ca.crt -subj "/C=US/ST=CA/L=SF/O=JITBot/CN=JIT Bot CA"

# Generate server private key
openssl genrsa -out server.key 2048

# Create certificate signing request config
cat > server.conf <<EOF
[req]
default_bits = 2048
prompt = no
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]
C = US
ST = CA
L = San Francisco
O = JIT Bot
CN = ${SERVICE_NAME}.${NAMESPACE}.svc

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
DNS.4 = ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local
EOF

# Generate certificate signing request
openssl req -new -key server.key -out server.csr -config server.conf

# Generate server certificate
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 365 -extensions v3_req -extfile server.conf

# Create or update Kubernetes secret
kubectl create secret tls ${SECRET_NAME} \
    --cert=server.crt \
    --key=server.key \
    --namespace=${NAMESPACE} \
    --dry-run=client -o yaml | kubectl apply -f -

# Get CA bundle for webhook configuration
CA_BUNDLE=$(base64 < ca.crt | tr -d '\n')

echo "Certificate created successfully!"
echo "CA Bundle (for webhook configuration):"
echo "$CA_BUNDLE"

# Update webhook configurations with CA bundle
if command -v kubectl &> /dev/null; then
    echo "Updating webhook configurations with CA bundle..."
    
    # Update validating webhook
    kubectl patch validatingadmissionwebhook jit-bot-validating-webhook \
        --type='merge' \
        -p="{\"spec\":{\"clientConfig\":{\"caBundle\":\"$CA_BUNDLE\"}}}" || echo "Failed to patch validating webhook"
    
    # Update mutating webhooks
    kubectl patch mutatingadmissionwebhook jit-bot-mutating-webhook \
        --type='merge' \
        -p="{\"spec\":{\"clientConfig\":{\"caBundle\":\"$CA_BUNDLE\"}}}" || echo "Failed to patch mutating webhook"
        
    kubectl patch mutatingadmissionwebhook jit-bot-job-mutating-webhook \
        --type='merge' \
        -p="{\"spec\":{\"clientConfig\":{\"caBundle\":\"$CA_BUNDLE\"}}}" || echo "Failed to patch job mutating webhook"
fi

# Cleanup
cd /
rm -rf "$TMP_DIR"

echo "Done! Webhook certificates have been generated and installed."
echo "The operator should now be able to serve webhook requests."