#!/bin/bash
#
# CI/CD Platform CLI Helper
#
# Usage: ./cicd.sh <base_url> <command> [args...]
#
# Commands:
#   apps                              List all apps
#   app <id>                          Get app details
#   releases <app_id>                 List releases
#   release <release_id>              Get release details
#   build <app_id> <sha> [branch]     Trigger build
#   build-status <release_id>         Get build status
#   build-logs <release_id>           Get build logs
#   deploy <release_id>               Deploy release
#   deployments <app_id>              Get deployment info
#   scale <app_id> <replicas>         Scale deployment
#   logs <app_id> <pod_name>          Get pod logs
#   describe <app_id> <pod_name>      Describe pod

set -e

BASE_URL="${1:-http://localhost:8080}"
COMMAND="${2:-help}"
shift 2 2>/dev/null || true

case "$COMMAND" in
  apps)
    curl -s "$BASE_URL/api/apps" | jq .
    ;;

  app)
    curl -s "$BASE_URL/api/apps/$1" | jq .
    ;;

  releases)
    curl -s "$BASE_URL/api/apps/$1/releases" | jq .
    ;;

  release)
    curl -s "$BASE_URL/api/releases/$1" | jq .
    ;;

  build)
    APP_ID="$1"
    SHA="$2"
    BRANCH="${3:-main}"
    curl -s -X POST "$BASE_URL/api/apps/$APP_ID/releases" \
      -H "Content-Type: application/json" \
      -d "{\"branch\": \"$BRANCH\", \"commit_sha\": \"$SHA\"}" | jq .
    ;;

  build-status)
    curl -s "$BASE_URL/api/releases/$1/build-status" | jq .
    ;;

  build-logs)
    curl -s "$BASE_URL/api/releases/$1/build-logs" | jq .
    ;;

  deploy)
    curl -s -X POST "$BASE_URL/api/releases/$1/deploy" \
      -H "Content-Type: application/json" \
      -d '{}' | jq .
    ;;

  deployments)
    curl -s "$BASE_URL/api/apps/$1/deployments" | jq .
    ;;

  scale)
    APP_ID="$1"
    REPLICAS="$2"
    curl -s -X POST "$BASE_URL/api/apps/$APP_ID/scale" \
      -H "Content-Type: application/json" \
      -d "{\"replicas\": $REPLICAS}" | jq .
    ;;

  logs)
    curl -s "$BASE_URL/api/apps/$1/pods/$2/logs" | jq .
    ;;

  describe)
    curl -s "$BASE_URL/api/apps/$1/pods/$2/describe" | jq .
    ;;

  ingress)
    curl -s "$BASE_URL/api/apps/$1/ingress" | jq .
    ;;

  health)
    curl -s "$BASE_URL/health" | jq .
    ;;

  help|*)
    echo "CI/CD Platform CLI Helper"
    echo ""
    echo "Usage: $0 <base_url> <command> [args...]"
    echo ""
    echo "Commands:"
    echo "  apps                              List all apps"
    echo "  app <id>                          Get app details"
    echo "  releases <app_id>                 List releases"
    echo "  release <release_id>              Get release details"
    echo "  build <app_id> <sha> [branch]     Trigger build"
    echo "  build-status <release_id>         Get build status"
    echo "  build-logs <release_id>           Get build logs"
    echo "  deploy <release_id>               Deploy release"
    echo "  deployments <app_id>              Get deployment info"
    echo "  scale <app_id> <replicas>         Scale deployment"
    echo "  logs <app_id> <pod_name>          Get pod logs"
    echo "  describe <app_id> <pod_name>      Describe pod"
    echo "  ingress <app_id>                  Get ingress info"
    echo "  health                            Health check"
    ;;
esac
