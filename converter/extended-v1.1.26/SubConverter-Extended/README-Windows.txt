SubConverter-Extended Windows portable package

Start the program with start.bat or start.ps1.

Configuration priority:
1. PREF_PATH environment variable
2. base\pref.toml
3. base\pref.yml
4. base\pref.ini

On first start, if no user configuration exists, the launcher creates one from
the matching example file. The default generated file is base\pref.toml from
base\pref.example.toml.

Existing configuration files are never overwritten by the launcher. To keep a
custom configuration outside this directory, set PREF_PATH to the target file
before starting the launcher.
