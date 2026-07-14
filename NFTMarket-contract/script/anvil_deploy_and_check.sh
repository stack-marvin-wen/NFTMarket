#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if ! command -v forge >/dev/null 2>&1; then
  echo "forge not found. Please install Foundry first."
  exit 1
fi
if ! command -v cast >/dev/null 2>&1; then
  echo "cast not found. Please install Foundry first."
  exit 1
fi
if ! command -v anvil >/dev/null 2>&1; then
  echo "anvil not found. Please install Foundry first."
  exit 1
fi

RPC_URL="${RPC_URL:-http://127.0.0.1:8545}"
ANVIL_PORT="${ANVIL_PORT:-8545}"
CHAIN_ID="${CHAIN_ID:-31337}"

# Anvil 默认第一个测试账户私钥
export DEPLOYER_PRIVATE_KEY="${DEPLOYER_PRIVATE_KEY:-0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80}"

# 可按需覆盖以下参数
export PROTOCOL_SHARE="${PROTOCOL_SHARE:-250}"
export EIP712_NAME="${EIP712_NAME:-EasySwap}"
export EIP712_VERSION="${EIP712_VERSION:-1}"
# export FINAL_OWNER="0xYourOwnerAddress"

ANVIL_STARTED_BY_SCRIPT=0
ANVIL_PID=""

cleanup() {
  if [[ "$ANVIL_STARTED_BY_SCRIPT" == "1" ]] && [[ -n "$ANVIL_PID" ]]; then
    kill "$ANVIL_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if ! cast chain-id --rpc-url "$RPC_URL" >/dev/null 2>&1; then
  echo "Starting local anvil on port $ANVIL_PORT (chain-id $CHAIN_ID)..."
  anvil --port "$ANVIL_PORT" --chain-id "$CHAIN_ID" >/tmp/easyswap-anvil.log 2>&1 &
  ANVIL_PID=$!
  ANVIL_STARTED_BY_SCRIPT=1

  for _ in {1..30}; do
    if cast chain-id --rpc-url "$RPC_URL" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done

  if ! cast chain-id --rpc-url "$RPC_URL" >/dev/null 2>&1; then
    echo "Failed to start anvil. Check /tmp/easyswap-anvil.log"
    exit 1
  fi
else
  echo "Detected running RPC at $RPC_URL, reusing it."
fi

echo "Deploying EasySwap and running acceptance checks..."
forge script script/DeployEasySwapLocalAnvil.s.sol:DeployEasySwapLocalAnvilScript \
  --rpc-url "$RPC_URL" \
  --broadcast \
  -vv

echo "Done. Local deploy + acceptance checks passed."
