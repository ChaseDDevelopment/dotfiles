return {
  "stevearc/conform.nvim",
  opts = function(_, opts)
    -- Ensure `opts.formatters_by_ft` exists
    opts.formatters_by_ft = opts.formatters_by_ft or {}

    -- Add Ruff for Python
    opts.formatters_by_ft.python = opts.formatters_by_ft.python or {}
    table.insert(opts.formatters_by_ft.python, "ruff")

    -- Configure Ruff with line length of 79
    opts.formatters.ruff = {
      prepend_args = { "--config", "line-length=79" },
    }
    -- Add CSharpier for C#
    opts.formatters_by_ft.cs = opts.formatters_by_ft.cs or {}
    table.insert(opts.formatters_by_ft.cs, "csharpier")
    -- Add Prettier for TypeScript and JavaScript
    opts.formatters_by_ft.typescript = opts.formatters_by_ft.typescript or {}
    table.insert(opts.formatters_by_ft.typescript, "prettier")

    opts.formatters_by_ft.javascript = opts.formatters_by_ft.javascript or {}
    table.insert(opts.formatters_by_ft.javascript, "prettier")

    opts.formatters.prettier = {}

    -- Add Rustfmt for Rust
    opts.formatters_by_ft.rust = opts.formatters_by_ft.rust or {}
    table.insert(opts.formatters_by_ft.rust, "rustfmt")
    opts.formatters.rustfmt = {}

    -- Add sql-formatter for SQL
    opts.formatters_by_ft.sql = opts.formatters_by_ft.sql or {}
    table.insert(opts.formatters_by_ft.sql, "sql-formatter")
    opts.formatters["sql-formatter"] = {}

    -- Add Stylua for Lua
    opts.formatters_by_ft.lua = opts.formatters_by_ft.lua or {}
    table.insert(opts.formatters_by_ft.lua, "stylua")
    opts.formatters.stylua = {}

    -- Add Prettier for YAML
    opts.formatters_by_ft.yaml = opts.formatters_by_ft.yaml or {}
    table.insert(opts.formatters_by_ft.yaml, "prettier")

    -- Add Prettier for Docker (Dockerfiles)
    opts.formatters_by_ft.dockerfile = opts.formatters_by_ft.dockerfile or {}
    table.insert(opts.formatters_by_ft.dockerfile, "prettier")

    -- Add Prettier for JSON
    opts.formatters_by_ft.json = opts.formatters_by_ft.json or {}
    table.insert(opts.formatters_by_ft.json, "prettier")
    return opts
  end,
}
