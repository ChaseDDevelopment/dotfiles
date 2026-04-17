if vim.fn.has("nvim-0.11") == 0 then
	vim.api.nvim_echo({
		{ "This config requires Neovim 0.11+. You have: " .. tostring(vim.version()) .. "\n", "ErrorMsg" },
	}, true, {})
	return
end

vim.loader.enable()
require("core/options")
require("core/keymaps")
require("core/autocmds")
vim.pack.add({
	"https://github.com/folke/tokyonight.nvim",
	"https://github.com/nvim-treesitter/nvim-treesitter",
	"https://github.com/folke/snacks.nvim",
	"https://github.com/folke/which-key.nvim",
	"https://github.com/williamboman/mason.nvim",
	"https://github.com/mason-org/mason-lspconfig.nvim",
	"https://github.com/WhoIsSethDaniel/mason-tool-installer.nvim",
	"https://github.com/stevearc/conform.nvim",
	"https://github.com/saghen/blink.cmp",
	"https://github.com/nvim-mini/mini.statusline",
	"https://github.com/nvim-mini/mini.animate",
	"https://github.com/stevearc/aerial.nvim",
	"https://github.com/nvim-lua/plenary.nvim",
	{ src = "https://github.com/ThePrimeagen/harpoon", version = "harpoon2" },
	"https://github.com/MunifTanjim/nui.nvim",
	"https://github.com/folke/noice.nvim",
	"https://github.com/nvim-tree/nvim-web-devicons",
	"https://github.com/lewis6991/gitsigns.nvim",
	"https://github.com/echasnovski/mini.surround",
	"https://github.com/echasnovski/mini.ai",
	"https://github.com/echasnovski/mini.pairs",
	"https://github.com/folke/trouble.nvim",
	"https://github.com/folke/flash.nvim",
	"https://github.com/lukas-reineke/indent-blankline.nvim",
	"https://github.com/nvim-treesitter/nvim-treesitter-context",
	"https://github.com/folke/todo-comments.nvim",
	"https://github.com/folke/lazydev.nvim",
	"https://github.com/MeanderingProgrammer/render-markdown.nvim",
	"https://github.com/mfussenegger/nvim-lint",
	"https://github.com/sindrets/diffview.nvim",
	"https://github.com/folke/persistence.nvim",
	"https://github.com/akinsho/toggleterm.nvim",
	"https://github.com/RRethy/vim-illuminate",
	"https://github.com/tris203/precognition.nvim",
	"https://github.com/Weyaaron/nvim-training",
	"https://github.com/coder/claudecode.nvim",
	"https://github.com/christoomey/vim-tmux-navigator",
})
require("plugins/tokyonight")
require("plugins/treesitter")
require("plugins/snacks")
require("plugins/which-key")
require("plugins/mason")
require("plugins/lsp")
require("plugins/conform")
require("plugins/blink")
require("plugins/noice")
require("plugins/statusline")
require("plugins/animate")
require("plugins/aerial")
require("plugins/harpoon")
require("plugins/gitsigns")
require("plugins/surround")
require("plugins/ai")
require("plugins/pairs")
require("plugins/trouble")
require("plugins/flash")
require("plugins/indent")
require("plugins/treesitter-context")
require("plugins/todo-comments")
require("plugins/lazydev")
require("plugins/render-markdown")
require("plugins/lint")
require("plugins/diffview")
require("plugins/persistence")
require("plugins/toggleterm")
require("plugins/illuminate")
require("plugins/precognition")
require("plugins/claudecode")
local ok, err = pcall(require, "plugins/training")
if not ok then
	vim.notify("nvim-training failed to load: " .. err, vim.log.levels.WARN)
end
