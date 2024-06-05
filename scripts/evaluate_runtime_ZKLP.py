import pandas as pd

# Path to read the data file
file_path = '../benchmarks/raw/bench_ZKLP32_G16_BN254.txt'

# Reading the data from the .txt file
df = pd.read_csv(file_path)

# Calculating descriptive statistics for the entire dataset
all_stats = df.describe()

# Grouping the data by 'Resolution' and calculating descriptive statistics
grouped_stats = df.groupby('Resolution').describe()

# Path to save the results
output_path = '../benchmarks/results/bench_ZKLP32_G16_BN254_results.txt'

# Saving the descriptive statistics to a .txt file
with open(output_path, 'w') as file:
    file.write("Descriptive Statistics for the Entire Dataset:\n")
    file.write(all_stats.to_string())
    file.write("\n\nDescriptive Statistics Grouped by Resolution:\n")
    file.write(grouped_stats.to_string())
