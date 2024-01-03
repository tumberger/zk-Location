import random
import struct

def cubic_polynomial(x):
    # Cubic polynomial: 3x³ + 2x² + x + 5
    return 3*x**3 + 2*x**2 + x + 5

def degree_10_polynomial(x):
    # Polynomial of degree 10
    return x**10 + 9*x**9 + 8*x**8 + 7*x**7 + 6*x**6 + 5*x**5 + 4*x**4 + 3*x**3 + 2*x**2 + x + 1

def generate_test_values(n, function, float_type='float64'):
    test_values = []

    if float_type == 'float32':
        pack_format = '>f'
        unpack_format = '>I'
    else:
        pack_format = '>d'
        unpack_format = '>Q'

    for _ in range(n):
        x = random.uniform(-100, 100)  # Generate a random float between -100 and 100
        result = function(x)

        x_hex = struct.unpack(unpack_format, struct.pack(pack_format, x))[0]
        result_hex = struct.unpack(unpack_format, struct.pack(pack_format, result))[0]

        test_values.append((x_hex, result_hex))
    return test_values

# Generate and write test values for the cubic polynomial (float64)
test_values_cubic_float64 = generate_test_values(1000, cubic_polynomial, 'float64')
with open('./f64/poly_cubic', 'w') as file:
    for x_hex, result_hex in test_values_cubic_float64:
        file.write(f"{x_hex:016X} {result_hex:016X}\n")

# Generate and write test values for the cubic polynomial (float32)
test_values_cubic_float32 = generate_test_values(1000, cubic_polynomial, 'float32')
with open('./f32/poly_cubic', 'w') as file:
    for x_hex, result_hex in test_values_cubic_float32:
        file.write(f"{x_hex:08X} {result_hex:08X}\n")

# Generate and write test values for the polynomial of degree 10 (float64)
test_values_degree10_float64 = generate_test_values(1000, degree_10_polynomial, 'float64')
with open('./f64/poly_degree10', 'w') as file:
    for x_hex, result_hex in test_values_degree10_float64:
        file.write(f"{x_hex:016X} {result_hex:016X}\n")

# Generate and write test values for the polynomial of degree 10 (float32)
test_values_degree10_float32 = generate_test_values(1000, degree_10_polynomial, 'float32')
with open('./f32/poly_degree10', 'w') as file:
    for x_hex, result_hex in test_values_degree10_float32:
        file.write(f"{x_hex:08X} {result_hex:08X}\n")
