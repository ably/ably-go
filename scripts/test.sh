#!/usr/bin/env bash
#
# A script to run the tests.

usage() {
  cat <<EOF
usage: $0 [-h|--help] [-p|--protocol PROTOCOL]

Run the ably-go tests.

OPTIONS:
  -h, --help                Show this message
  -p, --protocol PROTOCOL   Run tests using PROTOCOL (either 'application/json' or 'application/x-msgpack', default is to run both)
EOF
}

main() {
  local protocol=""

  # parse the flags
  while true; do
    case "$1" in
      -p | --protocol)
        if [[ -z "$2" ]]; then
          usage
          exit 1
        fi
        protocol="$2"
        shift 2
        ;;
      -h | --help)
        usage
        exit 0
        ;;
      *)
        break
        ;;
    esac
  done

  # run an individual protocol if requested, or all protocols
  if [[ -n "${protocol}" ]]; then
    run_tests "${protocol}"
  else
    for protocol in application/json application/x-msgpack; do
      run_tests "${protocol}"
    done
  fi
}

# run tests with a specific protocol
run_tests() {
  local protocol=$1

  echo "$(date +%H:%M:%S) - Running ably-go tests with protocol=${protocol}"

  ABLY_PROTOCOL="${protocol}" go test -p 1 -race -v ./...
}

main "$@"
