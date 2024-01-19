def read_test_cases(file_path):
    test_cases = {}
    with open(file_path, 'r') as file:
        for line in file:
            parts = line.split()
            if len(parts) > 2:
                # Convert hex to int
                resolution = int(parts[2], 16)
                test_cases[resolution] = test_cases.get(resolution, 0) + 1
    return test_cases

def read_test_results(file_path, test_cases):
    test_results = {res: ['green'] * count for res, count in test_cases.items()}
    current_test = 0
    current_resolution = 0
    skip_lines = 0

    with open(file_path, 'r') as file:
        for line in file:
            if skip_lines > 0:
                skip_lines -= 1
                continue
            if "--- PASS: TestLoc2Index32/bn254" in line:
                skip_lines = 2  # Skip the next two lines for a pass case'
                current_test += 1
                if current_test == test_cases[current_resolution]:
                    current_test = 0
                    current_resolution += 1
                    # continue

            elif "--- FAIL: TestLoc2Index32/" in line:
                current_test += 1
                test_results[current_resolution][current_test-1] = 'red'
                if current_test == test_cases[current_resolution]:
                    current_test = 0
                    current_resolution += 1
            
            if current_resolution > max(test_cases.keys()) and current_test == 9:
                break
    return test_results

def read_test_results_P6(file_path, test_cases):
    test_results = {res: ['green'] * count for res, count in test_cases.items()}
    current_test = 0
    current_resolution = 0
    skip_lines = 0

    with open(file_path, 'r') as file:
        for line in file:
            if skip_lines > 0:
                skip_lines -= 1
                continue
            if "--- PASS: TestLoc2IndexFromFile/bn254" in line:
                skip_lines = 2  # Skip the next two lines for a pass case'
                current_test += 1
                if current_test == test_cases[current_resolution]:
                    current_test = 0
                    current_resolution += 1
                    # continue

            elif "--- FAIL: TestLoc2IndexFromFile/" in line:
                current_test += 1
                test_results[current_resolution][current_test-1] = 'red'
                if current_test == test_cases[current_resolution]:
                    current_test = 0
                    current_resolution += 1
            
            if current_resolution > max(test_cases.keys()) and current_test == 9:
                break
    return test_results


def generate_latex_table(test_P6, test_P12, test_results32, test_results64, resolutions_per_row=5, tests_per_resolution=9):
    latex_code = "\\begin{table*}[!t]\n"
    latex_code += "\\centering\n"
    latex_code += "\\caption{Testing $\CircuitZKLP$ for resolutions $1$ to $15$ for single precision and double precision floating-point values. For a given resolution, test cases logarithmically approach the boundary of a given hexagon. If a cell is green, proof generation succeeds. If a cell is red, proof generation fails. All tests are executed for Groth16 over curve BN254.}\n"
    latex_code += "\\renewcommand{\\arraystretch}{0.8}\n"
    latex_code += "\\begin{tabularx}{\\textwidth}{l*{" + str(resolutions_per_row * tests_per_resolution) + "}{p{0.2mm}|}}\n"
    latex_code += "\\cmidrule(l){1-" + str(resolutions_per_row * tests_per_resolution + 1) + "}\n"
    latex_code += "& \\multicolumn{" + str(resolutions_per_row * tests_per_resolution) + "}{c}{\\textbf{Resolution}} \\\\\n"
    latex_code += "\\cmidrule(l){2-" + str(resolutions_per_row * tests_per_resolution + 1) + "}\n"

    for group in range(0, 15, resolutions_per_row):
        header_row = " & ".join([f"\\multicolumn{{{tests_per_resolution}}}{{c}}{{\\customarrowLeft{{4.2em}} \\textbf{{{res}}} \\customarrowRight{{4.2em}}}}" for res in range(group + 1, group + resolutions_per_row + 1)])
        latex_code += "& " + header_row + " \\\\\n"
        latex_code += "\\cmidrule(l){2-" + str(resolutions_per_row * tests_per_resolution + 1) + "}\n"
        
        for label, test_results in [("P6", test_P6), ("P12", test_P12), ("F32", test_results32), ("F64", test_results64)]:
            row_data = []
            for res in range(group + 1, group + resolutions_per_row + 1):
                colors = test_results.get(res, ['green'] * tests_per_resolution)
                row_data.extend([f"\\cellcolor{{{color}!25}}" for color in colors])
            latex_code += "\\textbf{" + label + "} " + " & " + " & ".join(row_data) + " \\\\\n"
            latex_code += "\\cmidrule(l){2-" + str(resolutions_per_row * tests_per_resolution + 1) + "}\n"

    latex_code += "\\end{tabularx}\n"
    latex_code += "\\end{table*}"
    return latex_code

test_casesP6 = read_test_cases('../data/f32/loc2index32.txt')
test_P6 = read_test_results_P6('../benchmarks/tests/loc2index_P6_G16_BN254.txt', test_casesP6)

test_casesP12 = read_test_cases('../data/f32/loc2index32.txt')
test_P12 = read_test_results_P6('../benchmarks/tests/loc2index_P12_G16_BN254.txt', test_casesP12)

test_cases32 = read_test_cases('../data/f32/loc2index32.txt')
test_results32 = read_test_results('../benchmarks/tests/loc2index32_g16_bn254.txt', test_cases32)

# print(test_results32)
test_cases64 = read_test_cases('../data/f64/loc2index64.txt')
test_results64 = read_test_results('../benchmarks/tests/loc2index64_g16_bn254.txt', test_cases32)
latex_table = generate_latex_table(test_P6, test_P12, test_results32, test_results64)
print(latex_table)


