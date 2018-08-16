xorm mysql generator
====

generate go struct from mysql

Usage: `./xorm_mysql_generator <dsn> <dir path> [table list]`

Example: `./xorm_mysql_generator "root:123456@(127.0.0.1:3306)/test" "user,address"`


What is done:

1. connect to mysql

2. show tables

3. for each table:

    1. desc table

    2. parse ach column, according to Field, Type, Key, Null, Default, Extra
