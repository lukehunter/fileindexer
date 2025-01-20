using System;
using System.Collections.Generic;
using System.Windows.Forms;
using Npgsql;

namespace FileTreeViewer
{
    public partial class Form1 : Form
    {
        private string connectionString;

        public Form1()
        {
            InitializeComponent();

            // Prompt for the database password
            string password = PromptForPassword();
            if (string.IsNullOrEmpty(password))
            {
                MessageBox.Show("Password is required to access the database.", "Error", MessageBoxButtons.OK, MessageBoxIcon.Error);
                Application.Exit();
                return;
            }

            connectionString = $"Host=localhost;Username=luke;Password={password};Database=files";
            LoadFileTree();
        }

        private string PromptForPassword()
        {
            using (var passwordForm = new PasswordPromptForm())
            {
                if (passwordForm.ShowDialog() == DialogResult.OK)
                {
                    return passwordForm.Password;
                }
            }
            return null;
        }

        private async void LoadFileTree()
        {
            // Show the progress dialog
            ProgressDialog progressDialog = new ProgressDialog();
            progressDialog.Show();

            try
            {
                // Fetch file paths in the background
                List<string> filePaths = await Task.Run(() => FetchFilePaths());

                // Partition the file paths for parallel processing
                int totalFiles = filePaths.Count;
                int chunkSize = Math.Max(1, totalFiles / Environment.ProcessorCount);
                var filePathChunks = Partition(filePaths, chunkSize);

                // Create a root node
                TreeNode rootNode = new TreeNode("Root");
                object lockObject = new object();

                int currentFile = 0;

                // Process chunks in parallel
                var tasks = filePathChunks.Select(chunk => Task.Run(() =>
                {
                    TreeNode localRoot = new TreeNode(); // Local root for this chunk
                    foreach (string path in chunk)
                    {
                        AddPathToTree(localRoot, path);

                        // Update progress safely
                        lock (lockObject)
                        {
                            currentFile++;
                            int progress = (int)((currentFile / (float)totalFiles) * 100);

                            // Update the progress dialog on the UI thread
                            progressDialog.Invoke(new Action(() =>
                            {
                                progressDialog.UpdateProgress(progress, $"Processing {currentFile}/{totalFiles} files...");
                            }));
                        }
                    }

                    // Merge the local root into the main root node
                    lock (lockObject)
                    {
                        foreach (TreeNode child in localRoot.Nodes)
                        {
                            rootNode.Nodes.Add((TreeNode)child.Clone());
                        }
                    }
                }));

                // Wait for all tasks to complete
                await Task.WhenAll(tasks);

                // Add the root node to the TreeView on the UI thread
                fileTreeView.Invoke(new Action(() =>
                {
                    fileTreeView.BeginUpdate();
                    fileTreeView.Nodes.Clear();
                    fileTreeView.Nodes.Add(rootNode);
                    rootNode.Expand();
                    fileTreeView.EndUpdate();
                }));
            }
            catch (Exception ex)
            {
                MessageBox.Show($"Error loading file tree: {ex.Message}", "Error", MessageBoxButtons.OK, MessageBoxIcon.Error);
            }
            finally
            {
                // Close the progress dialog
                progressDialog.Close();
            }
        }



        private static List<List<T>> Partition<T>(List<T> source, int chunkSize)
        {
            var chunks = new List<List<T>>();
            for (int i = 0; i < source.Count; i += chunkSize)
            {
                chunks.Add(source.GetRange(i, Math.Min(chunkSize, source.Count - i)));
            }
            return chunks;
        }



        private List<string> FetchFilePaths()
        {
            var filePaths = new List<string>();

            using (var connection = new NpgsqlConnection(connectionString))
            {
                connection.Open();

                // Bulk query to fetch all file paths
                using (var command = new NpgsqlCommand("SELECT filepath FROM file_hashes", connection))
                using (var reader = command.ExecuteReader())
                {
                    while (reader.Read())
                    {
                        filePaths.Add(reader.GetString(0));
                    }
                }
            }

            return filePaths;
        }


        private void AddPathToTree(TreeNode rootNode, string path)
        {
            string[] parts = path.Trim('/').Split('/');
            TreeNode currentNode = rootNode;

            foreach (string part in parts)
            {
                // Check if the node already exists at this level
                TreeNode[] existingNodes = currentNode.Nodes.Find(part, false);
                if (existingNodes.Length > 0)
                {
                    currentNode = existingNodes[0];
                }
                else
                {
                    TreeNode newNode = new TreeNode(part)
                    {
                        Name = part,
                        Tag = path  // Store full path for context menu actions
                    };
                    currentNode.Nodes.Add(newNode);
                    currentNode = newNode;
                }
            }
        }

    }
}
