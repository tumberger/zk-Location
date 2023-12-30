# This file generates test data for math functions (atan, ...)
# The test data contains 
# Column 1 - Input 
# Column 2 - Output (Real Float)
# Column 3 - Closest real float surrounding Output lower
# Column 4 - Closest real float surrounding Output upper

import math
import random
import struct

def find_surrounding_floats(r):
    r_minus = math.nextafter(r, -float('inf'))  # The largest float less than r
    r_plus = math.nextafter(r, float('inf'))    # The smallest float greater than r
    return r_minus, r_plus

def generate_random_float(iteration):
    # Define 10 different ranges
    ranges = [
        (1, 1e2),        # Moderate values
        (1e2, 1e10),        # Slightly larger values
        (1e10, 1e100),        # Even larger values
        (-1, -1e2),        # Moderate values
        (-1e2, -1e10),        # Slightly larger values
        (-1e10, -1e100),        # Even larger values
        (1, 1e-2),        # Moderate values
        (1e-2, 1e-10),        # Slightly larger values
        (1e-10, 1e-100),        # Even larger values
        (1e-100, 1e-200),        # Even larger values
    ]

    # Determine which range to use based on the current iteration
    range_index = iteration // 100  # Changes range every 100 iterations
    chosen_range = ranges[range_index]  # Selects appropriate range

    x = random.uniform(*chosen_range)
    return x

def generate_test_values(n):
    test_values = []
    for i in range(n):
        x = generate_random_float(i)
        atan_result = math.atan(x)

        # Convert the numbers to hex representation (IEEE 754 binary64 format)
        x_hex = struct.unpack('>Q', struct.pack('>d', x))[0]
        atan_hex = struct.unpack('>Q', struct.pack('>d', atan_result))[0]

        # Find surrounding floats for atan_result
        lower_surround, upper_surround = find_surrounding_floats(atan_result)
        lower_surround_hex = struct.unpack('>Q', struct.pack('>d', lower_surround))[0]
        upper_surround_hex = struct.unpack('>Q', struct.pack('>d', upper_surround))[0]

        test_values.append((x_hex, atan_hex, lower_surround_hex, upper_surround_hex))
    return test_values

# Generate 1000 test values
test_values = generate_test_values(1000)

# Write the test values to a file
with open('./f64/atan_ulp', 'w') as file:
    for x_hex, atan_hex, lower_surround_hex, upper_surround_hex in test_values:
        file.write(f"{x_hex:016X} {atan_hex:016X} {lower_surround_hex:016X} {upper_surround_hex:016X}\n")
