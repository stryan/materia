# Resources

Resources are the files that actually get installed, removed, or updated by Materia. They exist in the Materia Repository as part of [components](./components.md).

Resources are installed with the same permissions and ownership as the source.

There are several kinds of resources broken up into two categories: Quadlet and Data resources.

## Resource Kinds

Resource kind is determined by file type (with the exception of scripts, which can be defined in a component manifest).

### Quadlet Resources

These resources are installed into the Quadlet directory. Removing them does not remove the created Quadlet (container,volume,etc) on the host unless `cleanup` is enabled.

Container and Pod units will be restarted automatically when their associated files are updated. This behaviour can be controlled with the `NoRestart` component setting.

The following file types are quadlets: `.container`,`.volume`,`.pod`,`.network`,`.build`,`.image` and `.kube`.

### Data Resources

These are installed into the Materia data directory and consist of everything that *isn't* a Quadlet file.

All Data resources are installed to the data directory, but some are installed to other locations as indicated.

By default most Data Resources are considered generic `File` type. The following special exceptions are denoted by their file type:

#### Scripts
Scripts are resources that end in `.sh` OR are manually specified as a script in the Component Manifest.

Scripts can be designated as setup scripts or cleanup scripts with the `Settings.SetupScript` and `Settings.CleanupScript` component manifest options, respectively.

They are installed to the Scripts directory as well the Data directory. By default this is `/usr/local/bin`.

#### Services
Services are resources that are Systemd unit files. The following types are recognized: `.timer`,`.service`,`.socket`,`.mount`,`.device`,`.automount`,`.device`,`.slice`,`.scope`,`.swap`,`path`, and `.target`.

They are installed the Systemd directory as well as the Data directory. By default this is `/etc/systemd/system`.


## Templating Resources

Resources are treated as plain text by default but can also be customized with [Go Templates](https://pkg.go.dev/text/template).

Resources are denoted as templates with the `.gotmpl` suffix. This suffix will not be considered part of the Resource name. Only the last instance of this will be cut off; `conf.gotmpl.gotmpl` will be treated as a resource named `conf.gotmpl`.

The most common usage of Templates is to interpolate [Attributes](./attributes.md) into them with `{{ .attributeName }}`.

Materia also includes several "macros" to make writing resources easier. A full list can be found in the [reference page](./reference/materia-templates.5.md) but some common ones are:

`{{ m_dataDir "component name" }}` which returns the data directory for the component specified. Useful for referring to config file resources.

`{{ m_default "attribute name" "default" }}` which returns the attribute value if defined or the default value if not.
