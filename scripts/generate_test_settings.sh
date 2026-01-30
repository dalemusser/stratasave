#!/bin/bash

# Settings API test data generator
# Creates test settings for various users to test search functionality
#
# Usage: ./generate_test_settings.sh <base_url> <api_key>
# Example: ./generate_test_settings.sh http://localhost:8080 your-api-key

BASE_URL="${1:-http://localhost:8080}"
API_KEY="${2}"

if [ -z "$API_KEY" ]; then
  echo "Usage: $0 <base_url> <api_key>"
  echo "Example: $0 http://localhost:8080 your-api-key"
  exit 1
fi

echo "Generating test settings data..."
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

# Create settings for "ja" users
for user in "${JA_USERS[@]}"; do
  echo "Creating settings for: $user"
  curl -s -X POST "$BASE_URL/api/settings/save" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{
      \"user_id\": \"$user\",
      \"game\": \"$GAME\",
      \"settings_data\": {
        \"audio\": 0.8,
        \"graphics\": \"high\",
        \"language\": \"en\",
        \"notifications\": true
      }
    }" | head -c 100
  echo ""
done

# Create settings for other users
for user in "${OTHER_USERS[@]}"; do
  echo "Creating settings for: $user"
  curl -s -X POST "$BASE_URL/api/settings/save" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{
      \"user_id\": \"$user\",
      \"game\": \"$GAME\",
      \"settings_data\": {
        \"audio\": 0.5,
        \"graphics\": \"medium\",
        \"language\": \"en\",
        \"notifications\": false
      }
    }" | head -c 100
  echo ""
done

echo ""
echo "Done! Created settings for:"
echo "  - ${#JA_USERS[@]} users matching 'ja' search"
echo "  - ${#OTHER_USERS[@]} other users"
echo "  - Game: $GAME"
echo ""
echo "Test by searching for 'ja' in the Settings Browser"
