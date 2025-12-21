# Update Helper

An intermediary program used during app self-updates to replace one path with another. This is mainly to consider Go Windows users as it is not entirely necessary for Linux and Mac. For creating an external update program, there are advantages:

* Simple Backup and restore during update procedure
* Reliable file log as a created artefact for debugging purposes

## Usage

```
updater-helper <old_path> <new_path> <log_path>
```

Log path is optional, defaulting to working directory