using System;
using System.Windows.Forms;

namespace FileTreeViewer
{
    public partial class PasswordPromptForm : Form
    {
        public string Password { get; private set; }

        public PasswordPromptForm()
        {
            InitializeComponent();
        }

        private void btnOk_Click(object sender, EventArgs e)
        {
            Password = txtPassword.Text;
            DialogResult = DialogResult.OK;
            Close();
        }

        private void btnCancel_Click(object sender, EventArgs e)
        {
            DialogResult = DialogResult.Cancel;
            Close();
        }
    }
}
