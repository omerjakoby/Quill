from pymongo import MongoClient

# Your provided Atlas URI
ATLAS_URI = "mongodb+srv://Admin:XViC7TqV0SSavMt3@cluster0.ynuwtha.mongodb.net/"

try:
    client = MongoClient(ATLAS_URI)
    print("Successfully connected to MongoDB Atlas!")

    # Option 1: Accessing the default 'test' database (if not specified in URI)
    default_db = client.test
    print(f"\nDefault database name (from client.test): {default_db.name}")

    # Option 2: Explicitly select a database by name
    # Replace 'your_chosen_db_name' with the actual database you want to use or create.
    # For example, if you have a database named 'myApplicationDb'
    my_specific_db = client.myApplicationDb
    # Or using dictionary-style access:
    # my_specific_db = client["myApplicationDb"]
    print(f"Explicitly selected database name: {my_specific_db.name}")

    # Option 3: List all existing database names on your Atlas cluster
    # This requires the 'Admin' user to have 'readAnyDatabase' role.
    print("\nAll database names on the Atlas cluster:")
    for db_name in client.list_database_names():
        print(db_name)

except Exception as e:
    print(f"An error occurred: {e}")
finally:
    if 'client' in locals() and client:
        client.close()
        print("\nMongoDB Atlas connection closed.")