require("mason").setup()

-- has() mirrors the blink.cmp cargo-vs-lua fallback pattern in
-- lua/core/autocmds.lua: if the SDK binary isn't on PATH, don't ask
-- Mason to install an LSP/tool whose language won't work anyway.
local function has(bin)
	return vim.fn.executable(bin) == 1
end

local lsps = {
	"lua_ls",
	"yamlls",
	"dockerls",
	"docker_compose_language_service",
	"jsonls",
	"bashls",
	"taplo",
	"marksman",
	"html",
	"cssls",
}
if has("go") then
	table.insert(lsps, "gopls")
end
if has("rustc") then
	table.insert(lsps, "rust_analyzer")
end
if has("python3") then
	table.insert(lsps, "basedpyright")
end
if has("dotnet") then
	table.insert(lsps, "omnisharp")
end
if has("node") then
	table.insert(lsps, "vtsls")
end

require("mason-lspconfig").setup({
	ensure_installed = lsps,
	automatic_enable = false,
})

local tools = {
	"prettier",
	"stylua",
	"shfmt",
	"shellcheck",
	"markdownlint",
}
if has("go") then
	table.insert(tools, "goimports")
end
if has("python3") then
	table.insert(tools, "ruff")
	table.insert(tools, "sqlfluff")
end
if has("dotnet") then
	table.insert(tools, "csharpier")
end

require("mason-tool-installer").setup({
	ensure_installed = tools,
})
