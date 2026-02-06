## Reviewing rules

### Providing little proof

Everyone sometimes makes mistakes, you are the same. For every comment you leave in a MR review, provide "proof" in the form of a little code sample that proofs your point.

Rules for these code samples:

- The code samples should always be runnable using the programming language's tools (for example when writing something in Go, put it in `package main` and put it in a `main` function)
- Don't provide them when they are longer than 20 lines of code (without comments, logs, imports + any other things needed to get your code to run)
  - Keep the example as short as possible
- Always nicely format them to make sure it's easily understood
- Add comments to the code to explain what you are doing and why it proofs your point
- Put them in a spoiler so they don't make the scrolling experience worse, like this:

````md
<details>
  <summary>Spoiler warning</summary>
  
  Spoiler text. Note that it's important to have a space after the summary tag. You should be able to write any markdown you want inside the `<details>` tag... just make sure you close `<details>` afterward.
  
  ```javascript
  console.log("I'm a code block!");
  ```
  
</details>
````
