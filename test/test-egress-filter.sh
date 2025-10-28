#!/bin/bash
set -e

echo "=== Legion Router Egress Filter Test ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results
PASS=0
FAIL=0

test_result() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓ PASS${NC}: $2"
        ((PASS++))
    else
        echo -e "${RED}✗ FAIL${NC}: $2"
        ((FAIL++))
    fi
}

echo "Step 1: Create test network"
echo "----------------------------"
docker network create legion-test-net || echo "Network already exists"
echo ""

echo "Step 2: Start Legion Router container"
echo "--------------------------------------"
# Get absolute path to examples directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
EXAMPLES_DIR="$PROJECT_DIR/examples"

docker run -d \
    --name legion-router-test \
    --network legion-test-net \
    -v "$EXAMPLES_DIR:/examples" \
    legion-router:latest \
    /usr/local/bin/legion-router -config /examples/test-config.yaml

sleep 2

# Get the router IP (parse JSON output with jq)
ROUTER_IP=$(docker inspect legion-router-test | jq -r '.[0].networks[0].address' | cut -d'/' -f1)
echo "Legion Router IP: $ROUTER_IP"
echo ""

echo "Step 3: Start test app container"
echo "---------------------------------"
docker run -d \
    --name test-app \
    --network legion-test-net \
    alpine:latest \
    sleep 3600

sleep 2

echo ""
echo "Step 4: Install test tools in app container"
echo "--------------------------------------------"
docker exec test-app apk add --no-cache curl bind-tools

echo ""
echo "Step 5: Modify app container routes"
echo "------------------------------------"
# Save original route
docker exec test-app ip route show default
docker exec test-app ip route del default || true
docker exec test-app ip route add default via $ROUTER_IP
docker exec test-app ip route show
echo ""

echo "Step 5: Run connectivity tests"
echo "-------------------------------"

# Test 1: DNS should work to 8.8.8.8
echo -n "Test 1: DNS query to 8.8.8.8 (should PASS)... "
if docker exec test-app nslookup google.com 8.8.8.8 >/dev/null 2>&1; then
    test_result 0 "DNS to 8.8.8.8"
else
    test_result 1 "DNS to 8.8.8.8"
fi

# Test 2: HTTPS to google.com should work
echo -n "Test 2: HTTPS to google.com (should PASS)... "
if docker exec test-app timeout 5 curl -s -o /dev/null -w "%{http_code}" https://google.com 2>/dev/null | grep -q "^[23]"; then
    test_result 0 "HTTPS to google.com"
else
    test_result 1 "HTTPS to google.com"
fi

# Test 3: HTTPS to github.com should FAIL (not in allowlist)
echo -n "Test 3: HTTPS to github.com (should FAIL)... "
if docker exec test-app timeout 5 curl -s --max-time 3 https://github.com >/dev/null 2>&1; then
    test_result 1 "HTTPS to github.com should be blocked"
else
    test_result 0 "HTTPS to github.com blocked correctly"
fi

# Test 4: HTTP to google.com should FAIL (only HTTPS allowed)
echo -n "Test 4: HTTP to google.com (should FAIL)... "
if docker exec test-app timeout 5 curl -s --max-time 3 http://google.com >/dev/null 2>&1; then
    test_result 1 "HTTP to google.com should be blocked"
else
    test_result 0 "HTTP to google.com blocked correctly"
fi

echo ""
echo "Step 7: Check Legion Router logs"
echo "---------------------------------"
docker logs legion-router-test | tail -20

echo ""
echo "Step 8: Cleanup"
echo "---------------"
docker stop test-app legion-router-test
docker rm test-app legion-router-test
docker network rm legion-test-net

echo ""
echo "=== Test Summary ==="
echo -e "${GREEN}Passed: $PASS${NC}"
echo -e "${RED}Failed: $FAIL${NC}"
echo ""

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
