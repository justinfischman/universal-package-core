// A small demonstration entry that uses the library in ./lib.ts.
import { greet } from "./lib.ts";

console.log(greet(process.argv[2] ?? "world"));
