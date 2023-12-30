# This code is partially taken from https://github.com/DKenefake/OptimalPoly
from mpmath import mp
import numpy

# Setting the precision higher for more accurate calculations
mp.dps = 50

def bisection_search(f, low: float, high: float):
    if f(high) < f(low):
        low, high = high, low

    mid = .5 * (low + high)
    while True:
        if f(mid) < 0:
            low = mid
        else:
            high = mid
        mid = .5 * (high + low)

        if abs(high - low) < 10 ** (-(mp.dps / 2)):
            break

    return mid


def concave_max(f, low: float, high: float):
    scale = high - low
    h = mp.mpf('0.' + ''.join(['0' for i in range(int(mp.dps / 1.5))]) + '1') * scale
    df = lambda x: (f(x + h) - f(x - h)) / (2.0 * h)

    return bisection_search(df, low, high)


def chev_points(n: int, lower: float = -1, upper: float = 1):
    index = numpy.arange(1, n+1)
    range_ = abs(upper - lower)
    return [(.5 * (mp.cos((2*i-1)/(2*n)*mp.pi) + 1)) * range_ + lower for i in index]


def remez(func, n_degree: int, lower: float = -1, upper: float = 1, max_iter: int = 10):
    x_points = chev_points(n_degree + 2, lower, upper)

    A = mp.matrix(n_degree + 2)
    coeffs = numpy.zeros(n_degree + 2)

    for i in range(n_degree + 2):
        A[i, n_degree + 1] = (-1) ** (i + 1)

    for _ in range(max_iter):
        vander = numpy.polynomial.chebyshev.chebvander(x_points, n_degree)

        for i in range(n_degree + 2):
            for j in range(n_degree + 1):
                A[i, j] = vander[i, j]

        b = mp.matrix([func(x) for x in x_points])
        l = mp.lu_solve(A, b)

        coeffs = l[:-1]
        r_i = lambda x: (func(x) - numpy.polynomial.chebyshev.chebval(x, coeffs))

        interval_list = list(zip(x_points, x_points[1:]))
        intervals = [upper]
        intervals.extend([bisection_search(r_i, *i) for i in interval_list])
        intervals.append(lower)

        extermum_interval = [[intervals[i], intervals[i + 1]] for i in range(len(intervals) - 1)]
        extremums = [concave_max(r_i, *i) for i in extermum_interval]

        extremums[0] = mp.mpf(upper)
        extremums[-1] = mp.mpf(lower)

        errors = [abs(r_i(i)) for i in extremums]
        mean_error = numpy.mean(errors)

        if numpy.max([abs(error - mean_error) for error in errors]) < 0.000001 * mean_error:
            break

        x_points = extremums

    return [float(i) for i in numpy.polynomial.chebyshev.cheb2poly(coeffs)], float(mean_error)

# Approximating atan function
function = lambda x: mp.atan(x)
degree = 24  # Degree of the polynomial
lower = 0
upper = 1
poly_coeffs, max_error = remez(function, degree, lower, upper)

print("Polynomial Coefficients:", poly_coeffs)
print("Maximum Error:", max_error)

