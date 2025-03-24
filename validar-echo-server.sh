#!/bin/bash

SERVER_CONTAINER_NAME="server"
SERVER_PORT="12345"
NETWORK_NAME="tp0_testing_net"

TEST_MESSAGE="Hello, world!"

RESPONSE=$(docker run --rm --network $NETWORK_NAME alpine sh -c "echo '$TEST_MESSAGE' | nc -w 3 $SERVER_CONTAINER_NAME $SERVER_PORT")

if [ "$RESPONSE" == "$TEST_MESSAGE" ]; then
    echo "action: test_echo_server | result: success"
else
    echo "action: test_echo_server | result: fail"
fi
