# DMerc Analyzer

- DMerc Analyzer is a tool for analyzing and visualizing data from DMerc.
- It provides a user-friendly interface for exploring and understanding the data.
- DMerc Analyzer supports various data formats and can handle large datasets efficiently.
- It offers a range of features for data analysis, including filtering, sorting, and grouping.
- DMerc Analyzer also provides visualizations for data exploration, such as charts and graphs.
- It fetches data from an mail account
- The credentials are securely stored and used to authenticate with the mail account.
- It is smart to discover new data only in the mail account
- Propose any features I might have forgotten

## Non features

- It is an golang binary with an embedded web UI (runs a local HTTP server,
  opens the system's default browser — see MIGRATIONSPLAN.md; previously
  built with https://fyne.io/ as a UI framework, replaced in migration
  milestone M5, see docs/adr/0001-web-oberflaeche-statt-fyne.md)
- Is stored the mail account credentials securely
- generate a Taskfile with the common tasks for the project
- write tests
- generate a README.md with the common information for the project
- generate a CHANGELOG.md with the common information for the project
- Follow the SOLID principles
- Follow DDD
- Follow the Clean Architecture principles
- Follow the Clean Code principles
- Benutzerfreundliche Oberfläche für einfache Navigation und Bedienung