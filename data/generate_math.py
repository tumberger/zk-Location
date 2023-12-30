# This file generates test data for math functions (atan, ...)
# The test data contains 
# Column 1 - Input 
# Column 2 - Output (Real Float)
# Column 3 - Closest real float surrounding Output lower
# Column 4 - Closest real float surrounding Output upper
import math
import random
import struct
import numpy as np

def find_surrounding_floats(r, float_type='float64'):
    if float_type == 'float32':
        r = np.float32(r)
        r_minus = np.float32(np.nextafter(r, np.float32(-float('inf'))))  # The largest float less than r
        r_plus = np.float32(np.nextafter(r, np.float32(float('inf'))))    # The smallest float greater than r
    else:
        r_minus = math.nextafter(r, -float('inf'))  # The largest float less than r
        r_plus = math.nextafter(r, float('inf'))    # The smallest float greater than r
    return r_minus, r_plus

def generate_random_float(iteration, float_type='float64'):
    # Define 10 different ranges for float64
    ranges_float64 = [
        (1, 1e2),        # Moderate values
        (1e2, 1e10),     # Slightly larger values
        (1e10, 1e100),   # Even larger values
        (-1, -1e2),      # Moderate negative values
        (-1e2, -1e10),   # Larger negative values
        (-1e10, -1e100), # Even larger negative values
        (1e-2, 1),       # Moderate small values
        (1e-10, 1e-2),   # Smaller values
        (1e-100, 1e-10), # Even smaller values
        (1e-200, 1e-100) # Extremely small values
    ]

    # Define 10 different ranges for float32
    ranges_float32 = [
        (1, 100),         # Moderate values
        (100, 1e10),      # Larger values
        (1e10, 3.4e38),   # Near max range
        (-1, -100),       # Moderate negative values
        (-100, -1e10),    # Larger negative values
        (-1e10, -3.4e38), # Near min range
        (1e-2, 1),        # Small values
        (1e-10, 1e-2),    # Smaller values
        (1e-38, 1e-10),   # Near min positive range
        (0, 1e-38)        # Extremely small values
    ]

    ranges = ranges_float32 if float_type == 'float32' else ranges_float64

    # Determine which range to use based on the current iteration
    range_index = iteration // 100  # Changes range every 100 iterations
    chosen_range = ranges[range_index]  # Selects appropriate range

    x = random.uniform(*chosen_range)
    return x

def generate_test_values(n, function, float_type='float64'):
    test_values = []
    for i in range(n):
        x = generate_random_float(i, float_type)
        result = function(x)

        if float_type == 'float32':
            pack_format = '>f'
            unpack_format = '>I'
        else:
            pack_format = '>d'
            unpack_format = '>Q'

        x_hex = struct.unpack(unpack_format, struct.pack(pack_format, x))[0]
        result_hex = struct.unpack(unpack_format, struct.pack(pack_format, result))[0]

        lower_surround, upper_surround = find_surrounding_floats(result, float_type)
        lower_surround_hex = struct.unpack(unpack_format, struct.pack(pack_format, lower_surround))[0]
        upper_surround_hex = struct.unpack(unpack_format, struct.pack(pack_format, upper_surround))[0]

        test_values.append((x_hex, result_hex, lower_surround_hex, upper_surround_hex))
    return test_values

# Generate and write test values for atan (float64)
test_values_atan_float64 = generate_test_values(1000, math.atan, 'float64')
with open('./f64/atan_ulp', 'w') as file:
    for vals in test_values_atan_float64:
        file.write(' '.join(f"{val:016X}" for val in vals) + '\n')

# Generate and write test values for atan (float32)
test_values_atan_float32 = generate_test_values(1000, math.atan, 'float32')
with open('./f32/atan_ulp', 'w') as file:
    for vals in test_values_atan_float32:
        file.write(' '.join(f"{val:08X}" for val in vals) + '\n')

# Generate and write test values for sin (float64)
test_values_sin_float64 = generate_test_values(1000, math.sin, 'float64')
with open('./f64/sin_ulp', 'w') as file:
    for vals in test_values_sin_float64:
        file.write(' '.join(f"{val:016X}" for val in vals) + '\n')

# Generate and write test values for sin (float32)
test_values_sin_float32 = generate_test_values(1000, math.sin, 'float32')
with open('./f32/sin_ulp', 'w') as file:
    for vals in test_values_sin_float32:
        file.write(' '.join(f"{val:08X}" for val in vals) + '\n')