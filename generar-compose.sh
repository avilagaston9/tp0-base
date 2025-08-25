#!/bin/bash

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <output_file> <number_of_clients>"
    exit 1
fi

OUTPUT_FILE=$1
NUM_CLIENTS=$2

if ! [[ "$NUM_CLIENTS" =~ ^[0-9]+$ ]]; then
    echo "Error: The number of clients must be a positive integer."
    exit 1
fi

# Docker Compose file
## Server
cat <<EOF > $OUTPUT_FILE
name: tp0
services:
  server:
    container_name: server
    image: server:latest
    entrypoint: python3 /main.py
    environment:
      - PYTHONUNBUFFERED=1
    networks:
      - testing_net
    volumes:
      - ./server/config.ini:/config.ini

EOF

## Client services
for ((i=1; i<=$NUM_CLIENTS; i++))
do
    cat <<EOF >> $OUTPUT_FILE
  client$i:
    container_name: client$i
    image: client:latest
    entrypoint: /client
    environment:
      - CLI_ID=$i
      - NOMBRE=\${NOMBRE:-GASTON}
      - APELLIDO=\${APELLIDO:-AVILA}
      - DOCUMENTO=\${DOCUMENTO:-41780971}
      - NACIMIENTO=\${NACIMIENTO:-1999-03-19}
      - NUMERO=\${NUMERO:-9999}
    networks:
      - testing_net
    depends_on:
      - server
    volumes:
      - ./client/config.yaml:/config.yaml
      - ./.data:/.data

EOF
done

## Network configuration
cat <<EOF >> $OUTPUT_FILE
networks:
  testing_net:
    ipam:
      driver: default
      config:
        - subnet: 172.25.125.0/24
EOF

echo "Docker Compose file generated successfully: $OUTPUT_FILE"
