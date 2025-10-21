# Batch upload test scripts

Two ways to test batch uploads by calling the existing single-file endpoint sequentially.

## Bash script

1) Make executable:

```bash
chmod +x upload_dir.sh
```

2) Export environment variables and run:

```bash
export BASE_URL="http://localhost:8080"
export CLUSTER="demo"
export TOKEN="YOUR_TOKEN"
export NAMESPACE="default"
export POD="nginx-abc123"
export CONTAINER="nginx"
export TARGET_DIR="/tmp"
export LOCAL_DIR="$PWD/test-files"

mkdir -p "$LOCAL_DIR"
echo "hello1" > "$LOCAL_DIR/file1.txt"
echo "hello2" > "$LOCAL_DIR/file2.txt"

./upload_dir.sh
```

## Node.js script

1) Install dependencies:

```bash
npm install
```

2) Export environment variables and run:

```bash
export BASE_URL="http://localhost:8080"
export CLUSTER="demo"
export TOKEN="YOUR_TOKEN"
export NAMESPACE="default"
export POD="nginx-abc123"
export CONTAINER="nginx"
export TARGET_DIR="/tmp"
export LOCAL_DIR="$PWD/test-files"

mkdir -p "$LOCAL_DIR"
echo "hello1" > "$LOCAL_DIR/file1.txt"
echo "hello2" > "$LOCAL_DIR/file2.txt"

node uploadFiles.js
```

Both scripts print per-file results and a summary; a non-zero exit indicates failures.


