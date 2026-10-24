// Rookie: hunter - scan at random, shoot what is on your line.
function loop() {
  scan(pick(N, E, S, W));
  if (enemy == 3) fire(last);
}
