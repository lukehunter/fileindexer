import os
import psycopg2
import tkinter as tk
from tkinter import ttk, messagebox


# Database configuration
DB_NAME = "files"        # Replace with your database name
DB_USER = "luke"         # Replace with your username
DB_HOST = "localhost"    # Replace with your host


def fetch_file_paths():
    """Fetch all file paths from the database."""
    try:
        db_password = os.environ.get("DB_PASSWORD")
        if not db_password:
            raise ValueError("Database password not set. Please set the DB_PASSWORD environment variable.")

        connection = psycopg2.connect(
            dbname=DB_NAME,
            user=DB_USER,
            password=db_password,
            host=DB_HOST
        )
        cursor = connection.cursor()
        cursor.execute("SELECT filepath FROM file_hashes;")
        file_paths = cursor.fetchall()
        return [row[0] for row in file_paths]
    except Exception as e:
        messagebox.showerror("Database Error", f"Error: {e}")
        return []
    finally:
        if connection:
            cursor.close()
            connection.close()


def fetch_files_with_same_hash(file_path):
    """Fetch files with the same hash as the given file."""
    try:
        db_password = os.environ.get("DB_PASSWORD")
        if not db_password:
            raise ValueError("Database password not set. Please set the DB_PASSWORD environment variable.")

        connection = psycopg2.connect(
            dbname=DB_NAME,
            user=DB_USER,
            password=db_password,
            host=DB_HOST
        )
        cursor = connection.cursor()

        # Query to find files with the same hash
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
    finally:
        if connection:
            cursor.close()
            connection.close()


def build_tree(file_paths):
    """Build a nested dictionary representing the file tree."""
    tree = {}
    for path in file_paths:
        parts = path.strip('/').split('/')
        node = tree
        for part in parts:
            node = node.setdefault(part, {})
    return tree


class FileTreeApp:
    def __init__(self, root, file_tree):
        self.root = root
        self.root.title("File Tree Viewer")

        # TreeView setup
        self.tree = ttk.Treeview(root)
        self.tree.heading("#0", text="File Tree", anchor="w")
        self.tree.pack(fill="both", expand=True)

        # Right-click context menu
        self.context_menu = tk.Menu(self.root, tearoff=0)
        self.context_menu.add_command(label="Show Files with Same Hash", command=self.show_files_with_same_hash)

        # Bind events
        self.tree.bind("<Button-3>", self.show_context_menu)

        # Populate the TreeView
        self.populate_tree("", file_tree)

    def populate_tree(self, parent, node):
        """Recursively populate the TreeView with file tree data."""
        for key, value in node.items():
            item_id = self.tree.insert(parent, "end", text=key, open=False)
            if isinstance(value, dict):
                self.populate_tree(item_id, value)

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
        path_parts = []
        current_item = selected_item[0]
        while current_item:
            path_parts.insert(0, self.tree.item(current_item, "text"))
            current_item = self.tree.parent(current_item)
        file_path = "/" + "/".join(path_parts)

        # Fetch files with the same hash
        files = fetch_files_with_same_hash(file_path)

        # Display the result in a new window
        new_window = tk.Toplevel(self.root)
        new_window.title(f"Files with the Same Hash as: {file_path}")
        listbox = tk.Listbox(new_window)
        listbox.pack(fill="both", expand=True, padx=10, pady=10)

        for file in files:
            listbox.insert("end", file)


def main():
    # Fetch file paths and build the tree
    file_paths = fetch_file_paths()
    if not file_paths:
        return

    file_tree = build_tree(file_paths)

    # Create the GUI
    root = tk.Tk()
    app = FileTreeApp(root, file_tree)
    root.mainloop()


if __name__ == "__main__":
    main()
