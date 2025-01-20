using System;
using System.Windows.Forms;
using static System.Windows.Forms.VisualStyles.VisualStyleElement;

namespace FileTreeViewer
{
    public partial class ProgressDialog : Form
    {
        public ProgressDialog()
        {
            InitializeComponent();
        }

        public void UpdateProgress(int value, string message)
        {
            if (InvokeRequired)
            {
                Invoke(new Action(() => UpdateProgress(value, message)));
                return;
            }

            progressBar.Value = value;
            lblMessage.Text = message;
        }
    }
}
