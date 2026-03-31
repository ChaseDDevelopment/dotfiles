require('lint').linters_by_ft = {
	python = { 'ruff' },
	sh = { 'shellcheck' },
	bash = { 'shellcheck' },
	markdown = { 'markdownlint' },
	javascript = { 'eslint' },
	typescript = { 'eslint' },
	javascriptreact = { 'eslint' },
	typescriptreact = { 'eslint' },
}

vim.api.nvim_create_autocmd({ 'BufWritePost', 'BufReadPost', 'InsertLeave' }, {
	callback = function()
		require('lint').try_lint()
	end,
})
