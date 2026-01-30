// Migration script: Copy saves from 'mhs' collection to 'player_states'
// Run with: mongosh stratasave scripts/migrate_mhs_to_player_states.js
//
// Or connect first and run:
//   mongosh
//   use stratasave
//   load("scripts/migrate_mhs_to_player_states.js")

const SOURCE_COLLECTION = "mhs";
const TARGET_COLLECTION = "player_states";
const GAME_NAME = "mhs";

print("=== Migration: " + SOURCE_COLLECTION + " -> " + TARGET_COLLECTION + " ===");
print("");

// Check source collection exists and has documents
const sourceCount = db[SOURCE_COLLECTION].countDocuments();
print("Source collection '" + SOURCE_COLLECTION + "' has " + sourceCount + " documents");

if (sourceCount === 0) {
    print("No documents to migrate. Exiting.");
    quit(0);
}

// Check if target collection already has documents for this game
const existingCount = db[TARGET_COLLECTION].countDocuments({ game: GAME_NAME });
if (existingCount > 0) {
    print("WARNING: Target collection already has " + existingCount + " documents with game='" + GAME_NAME + "'");
    print("This script will skip documents that already exist (by _id).");
    print("");
}

// Migrate documents
print("Starting migration...");
print("");

let migrated = 0;
let skipped = 0;
let errors = 0;

const cursor = db[SOURCE_COLLECTION].find();

while (cursor.hasNext()) {
    const doc = cursor.next();

    // Build the new document, ensuring game field is set
    const newDoc = {
        _id: doc._id,
        user_id: doc.user_id,
        game: GAME_NAME,  // Set game to collection name
        timestamp: doc.timestamp,
        save_data: doc.save_data
    };

    try {
        // Use insertOne - will fail if _id already exists (idempotent)
        db[TARGET_COLLECTION].insertOne(newDoc);
        migrated++;

        if (migrated % 100 === 0) {
            print("  Migrated " + migrated + " documents...");
        }
    } catch (e) {
        if (e.code === 11000) {
            // Duplicate key error - document already exists
            skipped++;
        } else {
            print("ERROR migrating document " + doc._id + ": " + e.message);
            errors++;
        }
    }
}

print("");
print("=== Migration Complete ===");
print("  Migrated: " + migrated);
print("  Skipped (already exist): " + skipped);
print("  Errors: " + errors);
print("");

// Verify
const finalCount = db[TARGET_COLLECTION].countDocuments({ game: GAME_NAME });
print("Target collection now has " + finalCount + " documents with game='" + GAME_NAME + "'");

// Create index if it doesn't exist
print("");
print("Ensuring index on player_states...");
db[TARGET_COLLECTION].createIndex(
    { game: 1, user_id: 1, timestamp: -1 },
    { name: "idx_game_user_timestamp" }
);
print("Index ensured.");

print("");
print("Migration complete. You can now verify the data and optionally drop the '" + SOURCE_COLLECTION + "' collection:");
print("  db." + SOURCE_COLLECTION + ".drop()");
