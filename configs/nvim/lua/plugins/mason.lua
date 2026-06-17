require("mason").setup()

-- Single source of truth for which LSP servers to ensure. The list is
-- already capability-gated (node group needs npm, dev servers need their
-- SDK, uv-installed servers are excluded), so nothing doomed is attempted.
local servers = require("core/servers")

require("mason-lspconfig").setup({
	ensure_installed = servers.mason_ensure(),
	automatic_enable = false,
})

local function has(bin)
	return vim.fn.executable(bin) == 1
end

-- Formatters/linters. stylua/shfmt/shellcheck are zero-runtime binaries and
-- install anywhere; prettier/markdownlint are node-based (gate on npm).
-- Python tools (ruff/sqlfluff) and the Python LSPs are installed via
-- `uv tool install` by the installer, not Mason — uv brings its own
-- interpreter, so they need no system python/venv.
local tools = {
	"stylua",
	"shfmt",
	"shellcheck",
}
if has("npm") then
	table.insert(tools, "prettier")
	table.insert(tools, "markdownlint")
end
if has("go") then
	table.insert(tools, "goimports")
end
if has("dotnet") then
	table.insert(tools, "csharpier")
end

require("mason-tool-installer").setup({
	ensure_installed = tools,
})
