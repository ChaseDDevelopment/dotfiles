local function has(bin)
	return vim.fn.executable(bin) == 1
end

local linters_by_ft = {
	sh = { 'shellcheck' },
	bash = { 'shellcheck' },
	markdown = { 'markdownlint' },
}
-- ruff is installed via `uv tool install` (system PATH); gate on the tool,
-- not python3 (a uv-managed interpreter may not expose python3).
if has('ruff') then
	linters_by_ft.python = { 'ruff' }
end
if has('node') then
	linters_by_ft.javascript = { 'eslint' }
	linters_by_ft.typescript = { 'eslint' }
	linters_by_ft.javascriptreact = { 'eslint' }
	linters_by_ft.typescriptreact = { 'eslint' }
end

require('lint').linters_by_ft = linters_by_ft

vim.api.nvim_create_autocmd({ 'BufWritePost', 'BufReadPost', 'InsertLeave' }, {
	callback = function()
		require('lint').try_lint()
	end,
})
