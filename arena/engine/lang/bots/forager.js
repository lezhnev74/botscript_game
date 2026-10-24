// Forager: scans for coins and walks toward the one it saw, wanders to random spots when it sees none, runs when an enemy is close.
// num[0] = direction of the next look, num[1] = beat counter, num[2] = 1 + the way to a coin I saw (0 = none),
// pin[0] = the spot I am wandering to
function loop() {
  num[1]++;
  if (num[1] % 2 == 0) { look(); return; }       // every other beat: look around
  if (num[2] > 0) { move(num[2] - 1); return; }  // a coin was in that direction: go for it
  if (pin[0] == none || path(pin[0]) < 0) {      // no spot in mind (or reached it): pick one
    pin[0] = at(rand(1, WIDTH - 2), rand(1, HEIGHT - 2));
  }
  move(path(pin[0]));                            // no way there (-1) is a wait
}
function look() {
  if (num[2] > 0) num[0] = num[2] - 1;           // chasing a coin: look its way again
  else num[0] = right(num[0]);
  scan(num[0]);
  num[2] = coin > 0 ? num[0] + 1 : 0;            // still there? keep going, otherwise forget it
  if (enemy == 3 && loaded) fire(num[0]);        // a clear shot: take it
  else if (enemy >= 2 && !(loaded && hp >= 3)) { num[2] = 0; move(pick(left(last), right(last), back(last))); }   // too close and weak: run
}
