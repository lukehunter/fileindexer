import os
import psycopg2
import tkinter as tk
from tkinter import ttk, messagebox
import json

# Database configuration
DB_NAME = "files"        # Replace with your database name
DB_USER = "luke"         # Replace with your username
DB_HOST = "localhost"    # Replace with your host
CACHE_FILE = "cache.json"  # Cache file path

# Tag for files with no duplicates
SINGLE_TAG = "single"

def get_db_connection():
    """Get a database connection using environment variables."""
    db_password = os.environ.get("DB_PASSWORD")
    if not db_password:
        raise ValueError("Database password not set. Please set the DB_PASSWORD environment variable.")
    
    return psycopg2.connect(
        dbname=DB_NAME,
        user=DB_USER,
        password=db_password,
        host=DB_HOST
    )

def fetch_file_paths():
    """Fetch all file paths from the database."""
    try:
        with get_db_connection() as connection:
            with connection.cursor() as cursor:
                cursor.execute("SELECT filepath FROM file_hashes;")
                file_paths = cursor.fetchall()
                return [row[0] for row in file_paths]
    except Exception as e:
        messagebox.showerror("Database Error", f"Error: {e}")
        return []

def fetch_file_hash_counts():
    """Fetch the counts of files grouped by their hash."""
    try:
        with get_db_connection() as connection:
            with connection.cursor() as cursor:
                query = """
                SELECT hash, COUNT(*) AS file_count
                FROM file_hashes
                GROUP BY hash;
                """
                cursor.execute(query)
                hash_counts = cursor.fetchall()
                return {row[0]: row[1] for row in hash_counts}
    except Exception as e:
        messagebox.showerror("Database Error", f"Error: {e}")
        return {}

def fetch_files_with_same_hash(file_path):
    """Fetch files with the same hash as the given file."""
    try:
        with get_db_connection() as connection:
            with connection.cursor() as cursor:
                query = """
                SELECT f2.filepath
                FROM file_hashes f1
                JOIN file_hashes f2 ON f1.hash = f2.hash
                WHERE f1.filepath = %s AND f1.filepath != f2.filepath;
                """
                cursor.execute(query, (file_path,))
                files = cursor.fetchall()
                return [row[0] for row in files]
    except Exception as e:
        messagebox.showerror("Database Error", f"Error: {e}")
        return []

def fetch_all_file_hashes():
    """Fetch all file hashes from the database."""
    try:
        with get_db_connection() as connection:
            with connection.cursor() as cursor:
                cursor.execute("SELECT filepath, hash FROM file_hashes;")
                file_hashes = cursor.fetchall()
                return {row[0]: row[1] for row in file_hashes}
    except Exception as e:
        messagebox.showerror("Database Error", f"Error: {e}")
        return {}

def build_tree(file_paths):
    """Build a nested dictionary representing the file tree."""
    tree = {}
    for path in file_paths:
        parts = path.strip('/').split('/')
        node = tree
        for part in parts[:-1]:
            node = node.setdefault(part, {})
        node[parts[-1]] = None  # Use None to represent a file
    return tree

def load_cache():
    """Load cached data from the cache file."""
    if os.path.exists(CACHE_FILE):
        with open(CACHE_FILE, "r") as f:
            return json.load(f)
    return None

def save_cache(data):
    """Save data to the cache file."""
    with open(CACHE_FILE, "w") as f:
        json.dump(data, f)

class FileTreeApp:
    def __init__(self, root, file_tree, file_hash_counts, file_hashes):
        self.root = root
        self.file_hash_counts = file_hash_counts
        self.file_hashes = file_hashes
        self.root.title("File Tree Viewer")
        self.root.geometry("800x600")  # Set main window size to 800x600.

        # TreeView setup
        self.tree = ttk.Treeview(root, columns=("hash",))
        self.tree.heading("#0", text="File Tree", anchor="w")
        self.tree.heading("hash", text="Hash", anchor="w")
        self.tree.pack(fill="both", expand=True, side="left")

        # Add scrollbar to TreeView
        tree_scrollbar = ttk.Scrollbar(root, orient="vertical", command=self.tree.yview)
        self.tree.configure(yscrollcommand=tree_scrollbar.set)
        tree_scrollbar.pack(side="right", fill="y")

        # Right-click context menu
        self.context_menu = tk.Menu(self.root, tearoff=0)
        self.context_menu.add_command(label="Show Files with Same Hash", command=self.show_files_with_same_hash)
        self.context_menu.add_command(label="Show Unique Files", command=self.show_unique_files)

        # Bind events
        self.tree.bind("<Button-3>", self.show_context_menu)

        # Configure TreeView styles
        style = ttk.Style()
        style.configure("Treeview", background="white", fieldbackground="white")
        style.configure("Treeview.Item", background="white")
        style.map("Treeview.Item", background=[("selected", "blue")])
        style.configure("Treeview.Item.red", background="red")

        # Populate the TreeView
        self.populate_tree("", file_tree)

    def populate_tree(self, parent, node):
        """Recursively populate the TreeView with file tree data and apply highlights."""
        for key, value in node.items():
            item_id = self.tree.insert(parent, "end", text=key, values=(self.get_file_hash(self.get_full_path(parent, key)),), open=False)
            if isinstance(value, dict):
                self.populate_tree(item_id, value)

                # Check if the folder contains any red-highlighted files
                if self.is_folder_highlighted(item_id):
                    self.tree.item(item_id, tags=(SINGLE_TAG,))
            else:
                # Highlight individual files if they have a unique hash
                file_path = self.get_full_path(parent, key)
                file_hash = self.get_file_hash(file_path)
                if self.file_hash_counts.get(file_hash, 0) == 1:
                    self.tree.item(item_id, tags=(SINGLE_TAG,))
                    self.tree.tag_configure(SINGLE_TAG, background="red")

    def is_folder_highlighted(self, folder_id):
        """Check if a folder contains any red-highlighted files."""
        children = self.tree.get_children(folder_id)
        for child in children:
            if SINGLE_TAG in self.tree.item(child, "tags"):
                return True
        return False

    def get_full_path(self, parent, item_text):
        """Construct the full path of the selected file."""
        path_parts = []
        current_item = parent
        while current_item:
            path_parts.insert(0, self.tree.item(current_item, "text"))
            current_item = self.tree.parent(current_item)
        path_parts.append(item_text)
        return "/" + "/".join(path_parts)

    def get_file_hash(self, file_path):
        """Fetch the hash of a file given its path from the precached dictionary."""
        return self.file_hashes.get(file_path, None)

    def show_context_menu(self, event):
        """Display the context menu on right-click."""
        selected_item = self.tree.identify_row(event.y)
        if selected_item:
            self.tree.selection_set(selected_item)
            self.context_menu.post(event.x_root, event.y_root)

    def show_files_with_same_hash(self):
        """Open a new window with files sharing the same hash as the selected file."""
        selected_item = self.tree.selection()
        if not selected_item:
            return

        # Get the full path of the selected file
        file_path = self.get_full_path(selected_item[0])

        # Fetch files with the same hash
        files = fetch_files_with_same_hash(file_path)

        # Display the result in a new window
        new_window = tk.Toplevel(self.root)
        new_window.title(f"Files with the Same Hash as: {file_path}")
        new_window.geometry("600x400")  # Set pop-up window size to 600x400

        listbox = tk.Listbox(new_window)
        listbox.pack(fill="both", expand=True, side="left", padx=10, pady=10)

        # Add scrollbar to Listbox
        listbox_scrollbar = ttk.Scrollbar(new_window, orient="vertical", command=listbox.yview)
        listbox.configure(yscrollcommand=listbox_scrollbar.set)
        listbox_scrollbar.pack(side="right", fill="y")

        for file in files:
            listbox.insert("end", file)

    def show_unique_files(self):
        """Open a new window listing all files with only a single copy."""
        unique_files = sorted([path for path, hash in self.file_hashes.items() if self.file_hash_counts.get(hash, 0) == 1])

        # Display the result in a new window
        new_window = tk.Toplevel(self.root)
        new_window.title("Unique Files")
        new_window.geometry("600x400")  # Set pop-up window size to 600x400

        listbox = tk.Listbox(new_window)
        listbox.pack(fill="both", expand=True, side="left", padx=10, pady=10)

        # Add scrollbar to Listbox
        listbox_scrollbar = ttk.Scrollbar(new_window, orient="vertical", command=listbox.yview)
        listbox.configure(yscrollcommand=listbox_scrollbar.set)
        listbox_scrollbar.pack(side="right", fill="y")

        for file in unique_files:
            listbox.insert("end", file)

def main():
    # Load cached data
    cache = load_cache()
    if cache:
        file_paths = cache.get("file_paths", [])
        file_hash_counts = cache.get("file_hash_counts", {})
        file_hashes = cache.get("file_hashes", {})
    else:
        # Fetch data from the database
        file_paths = fetch_file_paths()
        if not file_paths:
            return

        file_hash_counts = fetch_file_hash_counts()
        file_hashes = fetch_all_file_hashes()

        # Save data to cache
        save_cache({
            "file_paths": file_paths,
            "file_hash_counts": file_hash_counts,
            "file_hashes": file_hashes
        })

    file_tree = build_tree(file_paths)

    # Create the GUI
    root = tk.Tk()
    app = FileTreeApp(root, file_tree, file_hash_counts, file_hashes)
    
    # Show the window listing all unique files at startup
    app.show_unique_files()
    
    root.mainloop()

if __name__ == "__main__":
    main()
