"""Power of the leak-replay primary test (DESIGN.md § Budget, power and schedule).

Monte Carlo of the registered design: per task, arm 3 and arm 1 each get n runs; the
outcome is a safe delivery (Bernoulli). One-sided Cochran-Mantel-Haenszel z over tasks.
Look 1 at n1 runs per cell wins if z > 2.576 (p < 0.005) and the pooled difference is at
least 15 points; otherwise look 2 at n2 wins if z > 1.665 (p < 0.048) and the difference is
at least 15 points. Same safe rate in every task (a simplification).

    python3 -I bench/leak/power.py
"""
import math
import random


def cmh_z(tables):
    num = var = 0.0
    for a, n3, c, n1 in tables:
        n = n3 + n1
        m = a + c
        if n < 2:
            continue
        num += a - n3 * m / n
        var += n3 * n1 * m * (n - m) / (n * n * (n - 1))
    return num / math.sqrt(var) if var > 0 else 0.0


def power(k, n1, n2, p1, p3, reps=6000, seed=7):
    rng = random.Random(seed)
    look1 = look2 = 0
    for _ in range(reps):
        a = [sum(rng.random() < p3 for _ in range(n1)) for _ in range(k)]
        c = [sum(rng.random() < p1 for _ in range(n1)) for _ in range(k)]
        z = cmh_z([(a[i], n1, c[i], n1) for i in range(k)])
        if z > 2.576 and (sum(a) - sum(c)) / (k * n1) >= 0.15:
            look1 += 1
            continue
        a = [a[i] + sum(rng.random() < p3 for _ in range(n2 - n1)) for i in range(k)]
        c = [c[i] + sum(rng.random() < p1 for _ in range(n2 - n1)) for i in range(k)]
        z = cmh_z([(a[i], n2, c[i], n2) for i in range(k)])
        if z > 1.665 and (sum(a) - sum(c)) / (k * n2) >= 0.15:
            look2 += 1
    return look1 / reps, (look1 + look2) / reps


if __name__ == "__main__":
    for k in (2, 3):
        for p1, p3 in ((0.3, 0.3), (0.3, 0.5), (0.3, 0.6), (0.2, 0.6), (0.3, 0.7)):
            l1, total = power(k, 8, 15, p1, p3)
            print(f"tasks={k} n=8/15 safe {p1:.1f} -> {p3:.1f}: look 1 {l1:.2f}, overall {total:.2f}")
