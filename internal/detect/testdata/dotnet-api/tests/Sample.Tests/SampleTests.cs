using Xunit;

public class SampleTests
{
    [Fact]
    public void Creates() { }

    [Fact]
    public void Updates() { }

    [Theory]
    [InlineData(1)]
    public void Validates(int n) { }
}
