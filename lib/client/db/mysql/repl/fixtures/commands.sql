-- temporarily disable note warnings for test consistency
set session sql_notes = 0;
create database if not exists test_commands;
create database if not exists foo;
create database if not exists bar;
set session sql_notes = 1;
use test_commands;

-- leading spaces/comments before commands
/* just a comment */
use foo;
use foo
     /*
llama
 */
use foo;
     /*
llama
 */
use foo
-- llama
use foo;
-- llama
use foo
  -- llama
    use foo;
  -- llama
    use foo
/* llama */use foo;
/* llama */use foo
/* llama */  use foo;
/* llama */  use foo
/* llama */
  use foo;
/* llama */
  use foo
/* llama
*/use foo;
/* llama
*/use foo
/* llama
*/  use foo;
/* llama
*/  use foo
/* llama
-- llama
*/use foo;
/* llama
-- llama
*/use foo
/* llama
-- llama
*/  use foo;
/* llama
-- llama
*/  use foo
/* llama
-- */use foo;
/* llama
-- */use foo
/* llama
-- */  use foo;
/* llama
-- */  use foo
/* llama */

use foo;use bar; use foo
-- this isn't a command, but that should just print an error message and help info.
\xyz select 1
\xyz;select 1;
delimiter $$
help$$
\d //
\h
help
status
\s
teleport
\t
use foo
\u mysql//
-- reset the delimiter to default ;
\d
teleport;\s;\h
use `bar`;drop database foo;drop database bar;drop database test_commands;quit
