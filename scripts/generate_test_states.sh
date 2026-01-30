#!/bin/bash

# States API test data generator
# Creates test states for various users to test search functionality
# Unlike settings, states accumulate (each save creates a new entry)
#
# Usage: ./generate_test_states.sh <base_url> <api_key>
# Example: ./generate_test_states.sh http://localhost:8080 your-api-key

BASE_URL="${1:-http://localhost:8080}"
API_KEY="${2}"

if [ -z "$API_KEY" ]; then
  echo "Usage: $0 <base_url> <api_key>"
  echo "Example: $0 http://localhost:8080 your-api-key"
  exit 1
fi

echo "Generating test states data..."
echo "Base URL: $BASE_URL"
echo ""

# Users that will match "ja" search
JA_USERS=(
  "james.wilson@example.com"
  "jane.doe@example.com"
  "jack.smith@example.com"
  "jasmine.lee@example.com"
  "jacob.brown@example.com"
)

# Users that will NOT match "ja" search
OTHER_USERS=(
  "bob.johnson@example.com"
  "alice.williams@example.com"
  "charlie.davis@example.com"
  "diana.miller@example.com"
  "edward.garcia@example.com"
  "fiona.martinez@example.com"
  "george.taylor@example.com"
  "helen.anderson@example.com"
  "ivan.thomas@example.com"
  "karen.white@example.com"
)

GAME="testgame"
STATES_PER_USER=3

# Create states for "ja" users (multiple states each)
for user in "${JA_USERS[@]}"; do
  for i in $(seq 1 $STATES_PER_USER); do
    echo "Creating state $i for: $user"
    curl -s -X POST "$BASE_URL/api/state/save" \
      -H "Authorization: Bearer $API_KEY" \
      -H "Content-Type: application/json" \
      -d "{
        \"user_id\": \"$user\",
        \"game\": \"$GAME\",
        \"save_data\": {
          \"level\": $i,
          \"score\": $((i * 1000)),
          \"health\": 100,
          \"inventory\": [\"sword\", \"shield\"]
        }
      }" | head -c 100
    echo ""
    sleep 0.1
  done
done

# Create states for other users (multiple states each)
for user in "${OTHER_USERS[@]}"; do
  for i in $(seq 1 $STATES_PER_USER); do
    echo "Creating state $i for: $user"
    curl -s -X POST "$BASE_URL/api/state/save" \
      -H "Authorization: Bearer $API_KEY" \
      -H "Content-Type: application/json" \
      -d "{
        \"user_id\": \"$user\",
        \"game\": \"$GAME\",
        \"save_data\": {
          \"level\": $i,
          \"score\": $((i * 500)),
          \"health\": 80,
          \"inventory\": [\"bow\", \"arrows\"]
        }
      }" | head -c 100
    echo ""
    sleep 0.1
  done
done

echo ""
echo "Done! Created states for:"
echo "  - ${#JA_USERS[@]} users matching 'ja' search ($STATES_PER_USER states each)"
echo "  - ${#OTHER_USERS[@]} other users ($STATES_PER_USER states each)"
echo "  - Total: $(( (${#JA_USERS[@]} + ${#OTHER_USERS[@]}) * STATES_PER_USER )) states"
echo "  - Game: $GAME"
echo ""
echo "Test by searching for 'ja' in the States Browser"
